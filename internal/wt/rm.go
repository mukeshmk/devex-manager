package wt

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mukeshmk/devex-manager/internal/config"
	"github.com/mukeshmk/devex-manager/internal/gitx"
)

// FindTarget matches git-wt-rm:62-91: folder suffix or exact branch, else a `target*` branch.
func FindTarget(wts []gitx.Worktree, repo *gitx.Repo, target string) (*gitx.Worktree, string) {
	for i, w := range wts {
		if w.Bare {
			continue
		}
		if strings.HasSuffix(w.Path, "/"+target) || (w.Branch != "" && w.Branch == target) {
			if w.Branch != "" {
				return &wts[i], w.Branch
			}
			return &wts[i], target
		}
	}
	if BranchExists(repo, target) {
		return nil, target
	}
	if out, err := gitx.Run(repo.CommonDir, "branch", "--list", target+"*", "--format=%(refname:short)"); err == nil && out != "" {
		first, _, _ := strings.Cut(out, "\n")
		return nil, first
	}
	return nil, target
}

// BranchExists reports whether refs/heads/<b> exists.
func BranchExists(repo *gitx.Repo, b string) bool {
	_, err := gitx.Run(repo.CommonDir, "show-ref", "--verify", "--quiet", "refs/heads/"+b)
	return err == nil
}

// Remove deletes wt (if it has a path) after unlinking its configured symlinks, then branch if it exists.
func Remove(repo *gitx.Repo, cfg config.RepoConfig, wt gitx.Worktree, branch string, out io.Writer) error {
	if wt.Path != "" {
		fmt.Fprintln(out, "[1/2] Removing worktree folder and configuration...")
		for _, p := range cfg.SymlinkPaths {
			full := filepath.Join(wt.Path, p)
			if fi, err := os.Lstat(full); err == nil && fi.Mode()&os.ModeSymlink != 0 {
				fmt.Fprintf(out, "  Removing symlink %s...\n", p)
				if err := os.Remove(full); err != nil {
					return err
				}
			}
		}
		if _, err := gitx.Run(repo.CommonDir, "worktree", "remove", wt.Path, "--force"); err != nil {
			return err
		}
	}
	if !BranchExists(repo, branch) {
		fmt.Fprintf(out, "Local branch '%s' does not exist.\n", branch)
		return nil
	}
	fmt.Fprintln(out, "[2/2] Deleting local branch...")
	_, err := gitx.Run(repo.CommonDir, "branch", "-D", branch)
	return err
}
