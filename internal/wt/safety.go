// Package wt holds the worktree deletion rules shared by `git wt rm`, `git wt clean` and the app.
package wt

import (
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/mukeshmk/devex-manager/internal/config"
	"github.com/mukeshmk/devex-manager/internal/gitx"
	"github.com/mukeshmk/devex-manager/internal/wtsync"
)

// Safety is what deleting a worktree and its branch would lose.
type Safety struct {
	Uncommitted                       []string
	Unpushed                          int
	Merged, UpstreamGone, HasUpstream bool
}

// Safe is the owner-approved rule: clean tree, and merged, upstream gone, or nothing unpushed.
func (s Safety) Safe() bool {
	return len(s.Uncommitted) == 0 && (s.Merged || s.UpstreamGone || s.Unpushed == 0)
}

// Assess inspects wt; remoteHeads is keyed by remote name, as fetched by the caller.
// A missing remote entry (e.g. ls-remote failed) means the upstream is not treated as gone.
func Assess(repo *gitx.Repo, cfg config.RepoConfig, wt gitx.Worktree, merged map[string]bool, remoteHeads map[string]map[string]bool) (Safety, error) {
	var s Safety
	var err error
	present := wt.Path != "" && !wt.Prunable
	if present {
		// One line per untracked file, so a configured path is told apart from a new file next to it.
		if s.Uncommitted, err = gitx.ChangedFiles(wt.Path, "--untracked-files=all"); err != nil {
			return s, err
		}
		s.Uncommitted = slices.DeleteFunc(s.Uncommitted, func(l string) bool { return configuredUntracked(l, wt.Path, cfg) })
	}
	ref, dir := wt.Branch, repo.CommonDir
	if ref == "" { // detached HEAD: count from the worktree itself
		if !present {
			return s, nil
		}
		ref, dir = "HEAD", wt.Path
	}
	if s.Unpushed, err = gitx.UnpushedCount(dir, ref); err != nil {
		return s, err
	}
	if wt.Branch != "" {
		s.Merged = merged[wt.Branch]
		var remote string
		remote, s.HasUpstream = repo.Upstream(wt.Branch)
		// Gone means the upstream's own remote branch (branch.<b>.merge) is missing; a "." remote is local.
		merge, mErr := gitx.Run(repo.CommonDir, "config", "--get", "branch."+wt.Branch+".merge")
		heads, fetched := remoteHeads[remote]
		s.UpstreamGone = s.HasUpstream && remote != "." && mErr == nil && merge != "" && fetched &&
			!heads[strings.TrimPrefix(merge, "refs/heads/")]
	}
	return s, nil
}

// configuredUntracked is true for an untracked porcelain line that add/sync put there: a file at or
// under a copy path (rm and clean refuse first unless those match main), or a symlink path that
// really is a symlink (Remove unlinks it; its target is untouched). Tracked changes always count.
func configuredUntracked(line, wtPath string, cfg config.RepoConfig) bool {
	rel, ok := strings.CutPrefix(line, "?? ")
	if !ok {
		return false
	}
	if u, err := strconv.Unquote(rel); err == nil { // git quotes unusual names
		rel = u
	}
	rel = strings.TrimSuffix(rel, "/")
	if wtsync.NotSynced(path.Base(rel)) { // sync never copies these, so rm's sync check can't see them
		return false
	}
	clean := func(p string) string { return strings.Trim(filepath.ToSlash(filepath.Clean(p)), "/") }
	for _, p := range cfg.CopyPaths {
		if p = clean(p); p != "" && p != "." && (rel == p || strings.HasPrefix(rel, p+"/")) {
			return true
		}
	}
	for _, p := range cfg.SymlinkPaths {
		if clean(p) == rel {
			fi, err := os.Lstat(filepath.Join(wtPath, rel))
			return err == nil && fi.Mode()&os.ModeSymlink != 0
		}
	}
	return false
}

// Inside reports whether cwd is path or below it, comparing whole path components.
func Inside(cwd, path string) bool {
	cwd, path = canon(cwd), canon(path)
	return cwd == path || strings.HasPrefix(cwd, path+string(os.PathSeparator))
}

// canon resolves symlinks, falling back to Clean when the path is gone.
func canon(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return filepath.Clean(p)
}

// SamePath compares two paths after symlink resolution.
func SamePath(a, b string) bool { return canon(a) == canon(b) }

// LostCommits is the unpushed count that deletion would actually lose (merged or gone means kept elsewhere).
func (s Safety) LostCommits() int {
	if s.Merged || s.UpstreamGone {
		return 0
	}
	return s.Unpushed
}

// GoneCommits counts local commits that Safe() lets go because the upstream is gone (owner rule:
// squash-merged elsewhere). They are still not on any remote, so callers show them as info.
func (s Safety) GoneCommits() int {
	if s.UpstreamGone && !s.Merged {
		return s.Unpushed
	}
	return 0
}

// RemoteHeads fetches heads once per distinct upstream remote of branches, for Assess.
// A remote whose ls-remote fails (offline) is left out, so its upstreams are not treated as gone.
func RemoteHeads(repo *gitx.Repo, branches ...string) map[string]map[string]bool {
	heads := map[string]map[string]bool{}
	tried := map[string]bool{}
	for _, b := range branches {
		remote, ok := repo.Upstream(b)
		if !ok || tried[remote] {
			continue
		}
		tried[remote] = true
		if h, err := repo.RemoteHeads(remote); err == nil {
			heads[remote] = h
		}
	}
	return heads
}
