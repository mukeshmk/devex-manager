package flows

import (
	"fmt"
	"path/filepath"

	"github.com/mukeshmk/devex-manager/internal/gitwt/sync"
	"github.com/mukeshmk/devex-manager/internal/gitwt/worktree"
	"github.com/mukeshmk/devex-manager/internal/shared/config"
	"github.com/mukeshmk/devex-manager/internal/shared/gitx"
	"github.com/mukeshmk/devex-manager/internal/shared/ui"
)

// Clean runs git wt clean. Every failure is printed to e.Err and returned as ErrReported.
func Clean(e Env, yes bool) error {
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
	if len(cfg.CopyPaths) > 0 {
		s, err := wtsync.Session(repo, cfg)
		if err != nil {
			return fail(err.Error())
		}
		if ok, _ := s.InSync(); !ok {
			return fail("Error: Cannot clean worktrees while files are not in sync.",
				"Please run "+ce.yellow.Render("git wt sync")+" to synchronize before deleting any worktrees.")
		}
	}

	fmt.Fprintln(out, co.blue.Render("Scanning for stale worktrees..."))
	cands, err := wt.FindStale(repo, cfg)
	if err != nil {
		return fail(err.Error())
	}
	if len(cands) == 0 {
		fmt.Fprintln(out, co.green.Render("✓ No stale worktrees found."))
		return nil
	}
	labels := make([]string, len(cands))
	pre := make([]bool, len(cands))
	for i, c := range cands {
		labels[i] = fmt.Sprintf("%s: %s (%s at %s)", filepath.Base(c.WT.Path), c.WT.Branch, c.Reason, c.WT.Path)
		if pre[i] = c.Safety.Safe(); !pre[i] {
			labels[i] += fmt.Sprintf("  ⚠ %d uncommitted file(s), %d commit(s) not on any remote",
				len(c.Safety.Uncommitted), c.Safety.LostCommits())
		}
		if n := c.Safety.GoneCommits(); n > 0 {
			labels[i] += fmt.Sprintf("  ℹ %d local commit(s) not on any remote", n)
		}
	}
	fmt.Fprintln(out, co.yellow.Render("Found the following stale worktrees:"))
	fmt.Fprintln(out, "\n"+co.red.Render("WARNING: This will permanently delete the worktree folder and the local branch."))
	var pr ui.Prompter = ui.DefaultPrompter{}
	if yes {
		for i, l := range labels { // the prompter would list them; --yes must still show what goes
			fmt.Fprintf(out, "  %d) %s\n", i+1, l)
		}
	} else {
		pr = e.Prompter
	}
	sel := pr.MultiSelect("Select worktrees to remove", labels, pre)

	picked := false
	for i, c := range cands {
		if !sel[i] {
			continue
		}
		picked = true
		if wt.Inside(cwd, c.WT.Path) {
			fmt.Fprintln(errw, ce.red.Render(fmt.Sprintf("Error: You are currently inside the worktree '%s' (%s) you are trying to delete!", c.WT.Branch, c.WT.Path)))
			fmt.Fprintln(errw, "Please 'cd' into a different folder and try again.")
			continue
		}
		fmt.Fprintln(out, co.blue.Render(fmt.Sprintf("Removing worktree %s...", c.WT.Branch)))
		if err := wt.Remove(repo, cfg, c.WT, c.WT.Branch, out); err != nil {
			return fail(err.Error())
		}
	}
	if !picked {
		fmt.Fprintln(out, "Cleanup cancelled.")
		return nil
	}
	fmt.Fprintln(out, co.green.Render("✓ Cleanup complete!"))
	return nil
}
