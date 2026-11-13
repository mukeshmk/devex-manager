package flows

import (
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mukeshmk/devex-manager/internal/gitwt/sync"
	"github.com/mukeshmk/devex-manager/internal/gitwt/worktree"
	"github.com/mukeshmk/devex-manager/internal/shared/config"
	"github.com/mukeshmk/devex-manager/internal/shared/gitx"
)

// Dirty is what Remove does with uncommitted changes when they are the only work at risk.
// Commits that are on no remote always refuse unless force.
type Dirty int

const (
	DirtyRefuse  Dirty = iota // refuse, as git wt rm does
	DirtyStash                // stash them, untracked files included, then remove
	DirtyDiscard              // throw them away with the worktree
)

// Remove runs git wt rm for one target. Every failure is printed to e.Err and returned as ErrReported.
func Remove(e Env, target string, force bool, dirty Dirty) error {
	out, errw := e.Out, e.Err
	co, ce := colours(out), colours(errw)
	fail := func(lines ...string) error {
		fmt.Fprintln(errw, ce.red.Render(lines[0]))
		for _, l := range lines[1:] {
			fmt.Fprintln(errw, l)
		}
		return ErrReported
	}
	cwd := e.Cwd
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

	stash := false
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
		var err error
		if stash, err = refuseUnsafe(repo, cfg, subject, branch, fail, dirty); err != nil {
			return err
		}
		// A detached worktree's HEAD says nothing about the same-named branch Remove also deletes.
		if w != nil && w.Branch == "" && wt.BranchExists(repo, branch) {
			if _, err := refuseUnsafe(repo, cfg, gitx.Worktree{Branch: branch}, branch, fail, DirtyRefuse); err != nil {
				return err
			}
		}
	}

	if stash {
		if err := stashChanges(*w, branch, cfg, out); err != nil {
			return fail(err.Error())
		}
		// Anything the stash left behind would go with the folder: refuse, keeping the stash.
		if _, err := refuseUnsafe(repo, cfg, *w, branch, fail, DirtyRefuse); err != nil {
			return err
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

// stashChanges stashes w's changes, untracked files included, leaving out the copy and symlink
// paths that add and sync put there, so the stash holds only the user's own work.
func stashChanges(w gitx.Worktree, branch string, cfg config.RepoConfig, out io.Writer) error {
	fmt.Fprintf(out, "Stashing uncommitted changes of %s...\n", filepath.Base(w.Path))
	args := []string{"stash", "push", "--include-untracked", "-m", fmt.Sprintf("git wt rm: %s (%s)", filepath.Base(w.Path), branch), "--", "."}
	for _, p := range append(slices.Clone(cfg.CopyPaths), cfg.SymlinkPaths...) {
		args = append(args, ":(exclude)"+p)
	}
	_, err := gitx.Run(w.Path, args...)
	return err
}

// refuseUnsafe lists what would be lost when the worktree or branch is not Safe. With dirty set and
// uncommitted changes the only risk, it lets the removal go on; stash reports that they need stashing.
func refuseUnsafe(repo *gitx.Repo, cfg config.RepoConfig, subject gitx.Worktree, branch string, fail func(...string) error, dirty Dirty) (stash bool, err error) {
	merged, _ := repo.MergedBranches(cfg.BaseBranch) // unknown base: treat nothing as merged
	s, err := wt.Assess(repo, cfg, subject, merged, wt.RemoteHeads(repo, branch))
	if err != nil {
		return false, fail(err.Error())
	}
	if s.Safe() {
		return false, nil
	}
	if dirty != DirtyRefuse && s.LostCommits() == 0 {
		return dirty == DirtyStash && subject.Path != "", nil
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
	return false, fail(append(lines, "Hint: use --force to remove anyway")...)
}
