package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mukeshmk/devex-manager/internal/gitwt/sync"
	"github.com/mukeshmk/devex-manager/internal/gitwt/worktree"
	"github.com/mukeshmk/devex-manager/internal/shared/config"
	"github.com/mukeshmk/devex-manager/internal/shared/gitx"
)

const rmUsage = "Usage: git wt rm [-f|--force] <folder-name-or-branch-name>"

func init() {
	var force bool
	cmd := &cobra.Command{
		Use:   "rm [-f|--force] <folder-name-or-branch-name>",
		Short: "Delete worktree folder & local branch",
		Args:  cobra.ArbitraryArgs,
		RunE: func(c *cobra.Command, args []string) error {
			return runRm(c.OutOrStdout(), c.ErrOrStderr(), args, force)
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "skip the sync and unsaved-work checks")
	registerWt(cmd)
}

func runRm(out, errw io.Writer, args []string, force bool) error {
	co, ce := colours(out), colours(errw)
	fail := func(lines ...string) error {
		fmt.Fprintln(errw, ce.red.Render(lines[0]))
		for _, l := range lines[1:] {
			fmt.Fprintln(errw, l)
		}
		return errExit1
	}
	switch {
	case len(args) == 0:
		return fail("Error: Target (folder or branch name) is required", rmUsage)
	case len(args) > 1:
		return fail("Error: Too many arguments specified", rmUsage)
	}
	target := args[0]
	cwd, err := os.Getwd()
	if err != nil {
		return fail(err.Error())
	}
	repo, err := gitx.Open(cwd)
	if err != nil {
		return fail(err.Error())
	}
	cfg, err := config.LoadRepo(repo.Root)
	if err != nil {
		return fail(err.Error())
	}
	wts, err := repo.Worktrees()
	if err != nil {
		return fail(err.Error())
	}
	w, branch := wt.FindTarget(wts, repo, target)
	if w == nil && !wt.BranchExists(repo, branch) {
		if similar := wt.BranchesWithPrefix(repo, target); len(similar) > 0 {
			return fail(fmt.Sprintf("No worktree or branch named %q. Did you mean: %s?", target, strings.Join(similar, ", ")))
		}
		fmt.Fprintln(errw, ce.yellow.Render(fmt.Sprintf("Worktree for '%s' not found (it may have already been removed).", target)))
		return fail(fmt.Sprintf("Local branch '%s' does not exist.", branch))
	}

	if w != nil {
		if wt.Inside(cwd, w.Path) {
			return fail("Error: You are currently inside the worktree you are trying to delete!",
				"Please 'cd' into a different folder (like your bare repo or main worktree) and try again.")
		}
		// Without a known main worktree the guard cannot run, so refuse instead of skipping it.
		main, _, err := wtsync.MainWorktree(repo, cfg, wts)
		if err != nil {
			return fail(err.Error())
		}
		if wt.SamePath(main, w.Path) {
			return fail(fmt.Sprintf("Error: Refusing to remove the main worktree '%s'.", filepath.Base(main)))
		}
	}
	if branch == cfg.BaseBranch {
		return fail(fmt.Sprintf("Error: Refusing to delete the base branch '%s'.", branch))
	}

	if !force {
		if len(cfg.CopyPaths) > 0 {
			s, err := wtsync.Session(repo, cfg)
			if err != nil {
				return fail(err.Error())
			}
			if ok, _ := s.InSync(); !ok {
				return fail("Error: Cannot remove worktree while files are not in sync.",
					"Please run "+ce.yellow.Render("git wt sync")+" to synchronize before deleting any worktrees.")
			}
		}
		subject := gitx.Worktree{Branch: branch}
		if w != nil {
			subject = *w
		}
		if err := refuseUnsafe(repo, cfg, subject, branch, fail); err != nil {
			return err
		}
		// A detached worktree's HEAD says nothing about the same-named branch Remove also deletes.
		if w != nil && w.Branch == "" && wt.BranchExists(repo, branch) {
			if err := refuseUnsafe(repo, cfg, gitx.Worktree{Branch: branch}, branch, fail); err != nil {
				return err
			}
		}
	}

	fmt.Fprintln(out, co.blue.Render("Cleaning up worktree and branch for: "+branch))
	var rmw gitx.Worktree
	if w != nil {
		rmw = *w
	} else {
		fmt.Fprintln(out, co.yellow.Render(fmt.Sprintf("Worktree for '%s' not found (it may have already been removed).", target)))
	}
	if err := wt.Remove(repo, cfg, rmw, branch, out); err != nil {
		return fail(err.Error())
	}
	fmt.Fprintln(out, "\n"+co.blue.Render("✓ Clean up complete! Workspace is tidy."))
	return nil
}

// refuseUnsafe lists what would be lost when the worktree or branch is not Safe.
func refuseUnsafe(repo *gitx.Repo, cfg config.RepoConfig, subject gitx.Worktree, branch string, fail func(...string) error) error {
	merged, _ := repo.MergedBranches(cfg.BaseBranch) // unknown base: treat nothing as merged
	s, err := wt.Assess(repo, cfg, subject, merged, wt.RemoteHeads(repo, branch))
	if err != nil {
		return fail(err.Error())
	}
	if s.Safe() {
		return nil
	}
	lines := []string{fmt.Sprintf("Error: Refusing to remove '%s': it has work that would be lost.", branch)}
	if n := len(s.Uncommitted); n > 0 {
		lines = append(lines, "Uncommitted changes:")
		for _, f := range s.Uncommitted[:min(n, 10)] {
			lines = append(lines, "  "+f)
		}
		if n > 10 {
			lines = append(lines, fmt.Sprintf("  ... and %d more", n-10))
		}
	}
	if n := s.LostCommits(); n > 0 {
		lines = append(lines, fmt.Sprintf("%d commit(s) not on any remote", n))
	}
	return fail(append(lines, "Hint: use --force to remove anyway")...)
}
