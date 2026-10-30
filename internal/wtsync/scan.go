// Package wtsync ports the scan and decision rules of git-wt-tools/git-wt-sync.
package wtsync

import (
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

// Stat is a file's presence, mtime (Unix seconds) and size (bytes).
type Stat struct {
	Exists      bool
	Mtime, Size int64
}

// FileState is one copy-path file across the manifest, main and every other worktree (keyed by abs path).
type FileState struct {
	Rel      string
	Recorded Stat
	Main     Stat
	Others   map[string]Stat
}

const conflictTag = ".devex-conflict-"

// ReadManifest parses `rel|mtime|size` lines; a missing file is an empty manifest.
func ReadManifest(path string) (map[string]Stat, error) {
	m := map[string]Stat{}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return m, nil
	}
	if err != nil {
		return nil, err
	}
	for _, l := range strings.Split(string(data), "\n") {
		// split on the last two '|' so a '|' in a filename survives
		i := strings.LastIndexByte(l, '|')
		if i < 0 {
			continue
		}
		j := strings.LastIndexByte(l[:i], '|')
		if j <= 0 {
			continue
		}
		mt, err1 := strconv.ParseInt(l[j+1:i], 10, 64)
		sz, err2 := strconv.ParseInt(l[i+1:], 10, 64)
		if err1 != nil || err2 != nil {
			continue
		}
		m[l[:j]] = Stat{Exists: true, Mtime: mt, Size: sz}
	}
	return m, nil
}

// WriteManifest writes entries as sorted `rel|mtime|size` lines, replacing the file atomically.
func WriteManifest(path string, entries map[string]Stat) error {
	var b strings.Builder
	for _, rel := range slices.Sorted(maps.Keys(entries)) {
		st := entries[rel]
		b.WriteString(rel + "|" + strconv.FormatInt(st.Mtime, 10) + "|" + strconv.FormatInt(st.Size, 10) + "\n")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(b.String()), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Scan returns the sorted union of regular files under copyPaths across main and others,
// with each worktree's Lstat. Symlinked dirs are not followed (like `find -type f`) and
// conflict markers are left out. Any read error aborts: a misread file would look deleted.
func Scan(main string, others []string, copyPaths []string, manifest map[string]Stat) ([]FileState, error) {
	all := append([]string{main}, others...)
	found := make([]map[string]Stat, len(all))
	errs := make([]error, len(all))
	var wg sync.WaitGroup
	for i, wt := range all {
		wg.Go(func() { found[i], errs[i] = walkWT(wt, copyPaths) })
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	union := map[string]bool{}
	for _, f := range found {
		for rel := range f {
			union[rel] = true
		}
	}
	files := make([]FileState, 0, len(union))
	for _, rel := range slices.Sorted(maps.Keys(union)) {
		fsx := FileState{Rel: rel, Recorded: manifest[rel], Others: map[string]Stat{}}
		for i, wt := range all {
			st, ok := found[i][rel]
			if !ok {
				// not walked here, but may still be a non-regular file (e.g. a symlink) that `[ -f ]` accepts
				var err error
				if st, err = lstat(filepath.Join(wt, rel)); err != nil {
					return nil, err
				}
			}
			if i == 0 {
				fsx.Main = st
			} else {
				fsx.Others[wt] = st
			}
		}
		files = append(files, fsx)
	}
	return files, nil
}

func lstat(path string) (Stat, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) || (err == nil && info.IsDir()) {
		return Stat{}, nil
	}
	if err != nil {
		return Stat{}, err
	}
	return Stat{Exists: true, Mtime: info.ModTime().Unix(), Size: info.Size()}, nil
}

func walkWT(wt string, copyPaths []string) (map[string]Stat, error) {
	out := map[string]Stat{}
	for _, p := range copyPaths {
		if p == "" {
			continue
		}
		root := filepath.Join(wt, p)
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if path == root && errors.Is(err, fs.ErrNotExist) {
					return nil
				}
				return err
			}
			if !d.Type().IsRegular() || strings.Contains(d.Name(), conflictTag) {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(wt, path)
			if err != nil {
				return err
			}
			out[filepath.ToSlash(rel)] = Stat{Exists: true, Mtime: info.ModTime().Unix(), Size: info.Size()}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// FindConflictMarkers lists `*.devex-conflict-*` entries in main inside each copy path,
// plus the `<path>.devex-conflict-*` siblings a file copy path gets. Sorted absolute paths.
func FindConflictMarkers(main string, copyPaths []string) []string {
	set := map[string]bool{}
	for _, p := range copyPaths {
		if p == "" {
			continue
		}
		root := filepath.Join(main, p)
		if ents, err := os.ReadDir(filepath.Dir(root)); err == nil {
			for _, e := range ents {
				if strings.HasPrefix(e.Name(), filepath.Base(root)+conflictTag) {
					set[filepath.Join(filepath.Dir(root), e.Name())] = true
				}
			}
		}
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil // unreadable: skipped, like `find 2>/dev/null`
			}
			if path != root && strings.Contains(d.Name(), conflictTag) {
				set[path] = true
			}
			return nil
		})
	}
	return slices.Sorted(maps.Keys(set))
}
