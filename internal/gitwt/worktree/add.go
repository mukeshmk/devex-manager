package wt

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mukeshmk/devex-manager/internal/shared/config"
	"github.com/mukeshmk/devex-manager/internal/shared/gitx"
)

// Add creates (or checks out) a worktree for branch and wires up symlinks and copies from main.
// Parity with git-wt-add:83-155.
func Add(repo *gitx.Repo, cfg config.RepoConfig, branch, customFolder string, out io.Writer) (string, error) {
	var folder string
	if cfg.NamingStrategy == "custom" {
		if customFolder == "" {
			return "", fmt.Errorf("Error: Custom folder name is required when naming_strategy is 'custom'\nUsage: git wt-add <branch-name> <custom-folder-name>")
		}
		folder = customFolder
	} else {
		var err error
		if folder, err = config.ResolveWorktreeName(branch, cfg.NamingStrategy); err != nil {
			return "", err
		}
	}
	main := filepath.Join(repo.Root, cfg.MainWorktreeName)
	path := filepath.Join(repo.Root, folder)
	fmt.Fprintf(out, "Setting up worktree for branch: %s\nFolder name will be: %s\n", branch, folder)

	fmt.Fprintln(out, "[1/3] Configuring git worktree...")
	_, e1 := gitx.Run(repo.Root, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	_, e2 := gitx.Run(repo.Root, "show-ref", "--verify", "--quiet", "refs/remotes/origin/"+branch)
	var err error
	if e1 == nil || e2 == nil {
		fmt.Fprintf(out, "  Branch '%s' already exists. Checking it out...\n", branch)
		_, err = gitx.Run(repo.Root, "worktree", "add", path, branch)
	} else {
		fmt.Fprintf(out, "  Branch '%s' does not exist. Creating it from '%s'...\n", branch, cfg.BaseBranch)
		_, err = gitx.Run(repo.Root, "worktree", "add", "-b", branch, path, cfg.BaseBranch)
	}
	if err != nil {
		return "", err
	}

	hasMain := isDir(main)
	fmt.Fprintln(out, "[2/3] Symlinking configuration directories...")
	if hasMain {
		for _, p := range cfg.SymlinkPaths {
			if p = strings.TrimSpace(p); p == "" || !inMain(main, p, out) {
				continue
			}
			fmt.Fprintf(out, "  Symlinking %s...\n", p)
			dst := filepath.Join(path, p)
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil { // nested path whose parent is untracked
				return "", err
			}
			if err := os.Symlink(filepath.Join(main, p), dst); err != nil {
				return "", err
			}
		}
	} else {
		fmt.Fprintf(out, "  Warning: Main worktree '%s' not found, skipping symlink creation\n", cfg.MainWorktreeName)
	}

	fmt.Fprintln(out, "[3/3] Copying paths from main worktree...")
	switch {
	case hasMain && len(cfg.CopyPaths) > 0:
		for _, p := range cfg.CopyPaths {
			if p = strings.TrimSpace(p); p == "" || !inMain(main, p, out) {
				continue
			}
			fmt.Fprintf(out, "  Copying %s...\n", p)
			if err := copyTree(filepath.Join(main, p), filepath.Join(path, p)); err != nil {
				return "", err
			}
		}
	case !hasMain && len(cfg.CopyPaths) > 0:
		fmt.Fprintf(out, "  Warning: Main worktree '%s' not found, skipping copy\n", cfg.MainWorktreeName)
	default:
		fmt.Fprintln(out, "  No copy paths configured, skipping.")
	}

	fmt.Fprintf(out, "\n✓ Worktree ready!\nTo switch to the new worktree, run:\n  cd \"%s\"\n", path)
	return path, nil
}

// RemoteBranchChoices turns `git branch -r` output into origin branch names that have no worktree.
func RemoteBranchChoices(raw string, wts []gitx.Worktree) []string {
	active := map[string]bool{}
	for _, w := range wts {
		active[w.Branch] = true
	}
	seen := map[string]bool{}
	var res []string
	for _, l := range strings.Split(raw, "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.Contains(l, "->") {
			continue
		}
		b := strings.TrimPrefix(l, "origin/")
		if !seen[b] && !active[b] {
			seen[b] = true
			res = append(res, b)
		}
	}
	sort.Strings(res)
	return res
}

// inMain reports whether p exists in the main worktree, saying so when it doesn't (usually a typo in .devex.conf).
func inMain(main, p string, out io.Writer) bool {
	if exists(filepath.Join(main, p)) {
		return true
	}
	fmt.Fprintf(out, "  Skipping %s: not found in main worktree\n", p)
	return false
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }
func isDir(p string) bool  { i, err := os.Stat(p); return err == nil && i.IsDir() }

// copyTree copies src to dst recursively like `cp -Rp`: modes and mtimes kept, symlinks not followed.
func copyTree(src, dst string) error {
	var dirs [][2]string // (path, src) fixed up after contents are written
	err := filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		d := filepath.Join(dst, rel)
		switch {
		case info.IsDir():
			dirs = append(dirs, [2]string{d, p})
			return os.MkdirAll(d, 0o755)
		case info.Mode()&os.ModeSymlink != 0:
			l, err := os.Readlink(p)
			if err != nil {
				return err
			}
			return os.Symlink(l, d)
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(d, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
		if err != nil {
			return err
		}
		_, err = io.Copy(out, in)
		if cerr := out.Close(); err == nil {
			err = cerr
		}
		if err == nil {
			err = os.Chmod(d, info.Mode().Perm())
		}
		if err == nil {
			err = os.Chtimes(d, info.ModTime(), info.ModTime())
		}
		return err
	})
	for i := len(dirs) - 1; i >= 0 && err == nil; i-- {
		si, e := os.Stat(dirs[i][1])
		if e != nil {
			return e
		}
		if err = os.Chmod(dirs[i][0], si.Mode().Perm()); err == nil {
			err = os.Chtimes(dirs[i][0], si.ModTime(), si.ModTime())
		}
	}
	return err
}
