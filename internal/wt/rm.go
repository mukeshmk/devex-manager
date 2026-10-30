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

// FindTarget picks the worktree target names, checking every worktree for each rule before the
// next: exact path, then folder name, then branch. A branch-only target returns a nil worktree.
// There is no prefix match: deleting a branch the user never named is worse than refusing.
func FindTarget(wts []gitx.Worktree, repo *gitx.Repo, target string) (*gitx.Worktree, string) {
	abs, _ := filepath.Abs(target)
	rules := []func(gitx.Worktree) bool{
		func(w gitx.Worktree) bool { return abs != "" && SamePath(abs, w.Path) },
		func(w gitx.Worktree) bool { return filepath.Base(w.Path) == target },
		func(w gitx.Worktree) bool { return w.Branch != "" && w.Branch == target },
	}
	for _, match := range rules {
		for i, w := range wts {
			if w.Bare || !match(w) {
				continue
			}
			if w.Branch != "" {
				return &wts[i], w.Branch
			}
			return &wts[i], filepath.Base(w.Path) // detached: rm also deletes a same-named branch
		}
	}
	return nil, target
}

// BranchesWithPrefix lists local branches starting with prefix, for a "did you mean" hint.
func BranchesWithPrefix(repo *gitx.Repo, prefix string) []string {
	out, err := gitx.Run(repo.CommonDir, "branch", "--list", prefix+"*", "--format=%(refname:short)")
	if err != nil || out == "" {
		return nil
	}
	return strings.Split(out, "\n")
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
