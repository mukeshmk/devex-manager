package wtsync

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/mukeshmk/devex-manager/internal/config"
	"github.com/mukeshmk/devex-manager/internal/gitx"
	"github.com/mukeshmk/devex-manager/internal/ui"
)

// SameFile compares two files by content (`diff -q`); unreadable means different.
func SameFile(a, b string) bool {
	x, err1 := os.ReadFile(a)
	y, err2 := os.ReadFile(b)
	return err1 == nil && err2 == nil && bytes.Equal(x, y)
}

// Resolve walks the plan, asking each Choice with pr, and returns the ops to apply in order.
// R4 conflicts are re-checked against the ops resolved so far: if earlier answers already made
// main and the worktree identical, the align ops are taken without asking (git-wt-sync:531).
func Resolve(p Plan, pr ui.Prompter) []Op {
	return walk(p.Steps, func(s Step, done []Op) int {
		c := s.Choice
		if c.RecheckSame != nil && sameAfter(done, c.RecheckSame[0], c.RecheckSame[1]) {
			return 0
		}
		if c.YesNo {
			if pr.Confirm(c.Question, c.Default == 0) {
				return 0
			}
			return 1
		}
		return pr.Choose(c.Question, c.Options, c.Default)
	}, nil)
}

// sameAfter reports whether a and b would have the same content once done is applied.
// Nothing is on disk yet, so each path is traced back to the file its content comes from.
func sameAfter(done []Op, a, b Loc) bool {
	const gone = "\x00"
	origin := map[string]string{}
	from := func(p string) string {
		if o, ok := origin[p]; ok {
			return o
		}
		return p
	}
	for _, op := range done {
		if op.Kind == Copy {
			origin[op.Dst.Path()] = from(op.Src.Path())
		} else {
			origin[op.Dst.Path()] = gone
		}
	}
	x, y := from(a.Path()), from(b.Path())
	if x == gone || y == gone {
		return false
	}
	return x == y || SameFile(x, y)
}

func (op Op) String() string {
	if op.Kind == Delete {
		return "delete " + op.Dst.Path()
	}
	return "copy " + op.Src.Path() + " -> " + op.Dst.Path()
}

// Apply runs ops strictly in order. Before an op touches an existing destination, that file is
// copied to backupRoot/<worktree folder>/<rel> (dir created only when needed). A Copy with a
// missing source is an error. Stops at the first error.
func Apply(ops []Op, backupRoot string) error {
	for _, op := range ops {
		if err := applyOne(op, backupRoot); err != nil {
			return fmt.Errorf("Error: sync stopped at %s: %w (backups in %s)", op, err, backupRoot)
		}
	}
	return nil
}

func applyOne(op Op, backupRoot string) error {
	dst := op.Dst.Path()
	if op.Kind == Copy {
		// checked before the backup so a bad op leaves nothing behind
		if _, err := os.Stat(op.Src.Path()); err != nil {
			return fmt.Errorf("source missing: %w", err)
		}
	}
	info, err := os.Lstat(dst)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		if op.Kind == Delete {
			return nil
		}
	case err != nil:
		return err
	case info.IsDir():
		return fmt.Errorf("%s is a directory", dst)
	default:
		bk := filepath.Join(backupRoot, filepath.Base(op.Dst.WT), filepath.FromSlash(op.Dst.Rel))
		// keep the first backup: a later overwrite in the same run only replaces content this run wrote
		if _, err := os.Lstat(bk); errors.Is(err, fs.ErrNotExist) {
			if err := copyFile(dst, bk); err != nil {
				return fmt.Errorf("backup failed: %w", err)
			}
		}
	}
	if op.Kind == Delete {
		return os.Remove(dst)
	}
	return copyFile(op.Src.Path(), dst)
}

// copyFile writes src to dst atomically (temp file in dst's dir, then rename), keeping mode and mtime.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), "."+filepath.Base(dst)+".devex-tmp-")
	if err != nil {
		return err
	}
	_, err = io.Copy(tmp, in)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chmod(tmp.Name(), info.Mode().Perm())
	}
	if err == nil {
		err = os.Chtimes(tmp.Name(), info.ModTime(), info.ModTime())
	}
	if err == nil {
		err = os.Rename(tmp.Name(), dst)
	}
	if err != nil {
		os.Remove(tmp.Name())
	}
	return err
}

// PruneEmptyDirs removes empty directories under each copy path (deepest first), like
// `find <wt>/<p> -depth -type d -empty -delete` (git-wt-sync:628-643).
func PruneEmptyDirs(worktrees, copyPaths []string) {
	for _, wt := range worktrees {
		for _, p := range copyPaths {
			if p == "" {
				continue
			}
			var dirs []string
			filepath.WalkDir(filepath.Join(wt, p), func(path string, d fs.DirEntry, err error) error {
				if err == nil && d.IsDir() {
					dirs = append(dirs, path)
				}
				return nil
			})
			slices.Reverse(dirs) // children before parents
			for _, d := range dirs {
				os.Remove(d) // fails harmlessly on non-empty dirs
			}
		}
	}
}

// SyncSession is a scanned repo ready for --check or planning.
type SyncSession struct {
	Main      string
	Others    []string
	Files     []FileState
	Markers   []string
	Root      string
	CopyPaths []string
	CommonDir string
}

// Session finds the main worktree (git-wt-sync:66-95), checks for conflict markers and scans.
func Session(repo *gitx.Repo, cfg config.RepoConfig) (*SyncSession, error) {
	wts, err := repo.Worktrees()
	if err != nil {
		return nil, err
	}
	s := &SyncSession{Root: repo.Root, CopyPaths: cfg.CopyPaths, CommonDir: repo.CommonDir}
	var active []string
	for _, w := range wts {
		if w.Bare || w.Prunable {
			continue
		}
		p, err := filepath.EvalSymlinks(w.Path)
		if err != nil {
			return nil, err
		}
		active = append(active, p)
		if filepath.Base(p) == cfg.MainWorktreeName {
			s.Main = p
		}
	}
	if s.Main == "" {
		if fi, err := os.Stat(filepath.Join(repo.Root, cfg.MainWorktreeName)); err == nil && fi.IsDir() {
			if s.Main, err = filepath.EvalSymlinks(filepath.Join(repo.Root, cfg.MainWorktreeName)); err != nil {
				return nil, err
			}
		} else if len(active) > 0 {
			s.Main = active[0]
		} else {
			return nil, errors.New("Error: No active worktrees found.")
		}
	}
	for _, p := range active {
		if p != s.Main {
			s.Others = append(s.Others, p)
		}
	}
	s.Markers = FindConflictMarkers(s.Main, s.CopyPaths)
	manifest, err := ReadManifest(s.ManifestPath())
	if err != nil {
		return nil, err
	}
	if s.Files, err = Scan(s.Main, s.Others, s.CopyPaths, manifest); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *SyncSession) ManifestPath() string {
	return filepath.Join(s.CommonDir, "info", "devex", "sync-manifest")
}

// InSync is the in-process `git wt sync --check`; unresolved conflict markers count as out of sync.
func (s *SyncSession) InSync() (bool, []string) {
	if len(s.Markers) > 0 {
		var lines []string
		for _, m := range s.Markers {
			lines = append(lines, "Unresolved conflict: "+m)
		}
		return false, lines
	}
	lines := CheckOutOfSync(s.Main, s.Others, s.Files, SameFile)
	return len(lines) == 0, lines
}

// Plan builds the sync plan for the scanned state.
func (s *SyncSession) Plan() Plan { return BuildPlan(s.Main, s.Others, s.Files, SameFile) }

// RebuildManifest records main's current files under the copy paths (git-wt-sync:645-664).
func (s *SyncSession) RebuildManifest() error {
	files, err := walkWT(s.Main, s.CopyPaths)
	if err != nil {
		return err
	}
	return WriteManifest(s.ManifestPath(), files)
}
