package flows

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/mukeshmk/devex-manager/internal/gitwt/sync"
	"github.com/mukeshmk/devex-manager/internal/shared/config"
	"github.com/mukeshmk/devex-manager/internal/shared/gitx"
	"github.com/mukeshmk/devex-manager/internal/shared/ui"
)

type SyncOpts struct{ Check, DryRun, Yes bool }

// Sync runs git wt sync. Every failure is printed to e.Err and returned as ErrReported.
func Sync(e Env, o SyncOpts) error {
	out, errw := e.Out, e.Err
	check, dryRun, yes := o.Check, o.DryRun, o.Yes
	co, ce := colours(out), colours(errw)
	fail := func(err error) error {
		fmt.Fprintln(errw, ce.red.Render(err.Error()))
		return ErrReported
	}
	repo, err := gitx.Open(e.Cwd)
	if err != nil {
		return fail(err)
	}
	cfg, err := config.LoadRepo(repo.Root)
	if err != nil {
		return fail(err)
	}
	if len(cfg.CopyPaths) == 0 {
		fmt.Fprintln(out, co.yellow.Render("No copy paths configured in .devex.conf (copies.paths). Nothing to sync."))
		return nil
	}
	s, err := wtsync.Session(repo, cfg)
	if err != nil {
		return fail(err)
	}
	if len(s.Markers) > 0 {
		fmt.Fprintln(errw, ce.red.Render("Sync blocked: Unresolved conflicts detected. Resolve the files, delete the conflict markers, and try again."))
		fmt.Fprintln(errw, ce.yellow.Render("Unresolved conflict files:"))
		for _, m := range s.Markers {
			if rel, ok := strings.CutPrefix(m, repo.Root+string(filepath.Separator)); ok {
				fmt.Fprintf(errw, "  - %s (at: %s)\n", rel, m)
			} else {
				fmt.Fprintf(errw, "  - %s\n", m)
			}
		}
		return ErrReported
	}

	header := "Synchronizing configuration copies..."
	if check {
		header = "Checking sync status of configuration copies..."
	}
	fmt.Fprintln(out, co.blue.Render(header))
	fmt.Fprintln(out, "Main worktree: "+co.green.Render(filepath.Base(s.Main)))

	if check {
		ok, lines := s.InSync()
		for _, l := range lines {
			if strings.HasPrefix(l, "Out of sync:") {
				l = co.yellow.Render(l)
			}
			fmt.Fprintln(out, l)
		}
		if !ok {
			fmt.Fprintln(out, "\n"+co.red.Render("Configuration files are not in sync."))
			return ErrReported
		}
		fmt.Fprintln(out, co.green.Render("✓ Everything is in sync."))
		return nil
	}

	plan := s.Plan()
	if len(plan.Steps) == 0 {
		fmt.Fprintln(out, co.green.Render("✓ Everything is in sync."))
	}
	for _, st := range plan.Steps {
		line := "  " + st.Summary
		if st.Choice != nil {
			line = co.yellow.Render(line)
		}
		fmt.Fprintln(out, line)
	}
	if dryRun {
		return nil
	}

	var pr ui.Prompter = ui.DefaultPrompter{}
	if e.TTY {
		pr = e.Prompter
		if len(plan.Steps) > 0 && !yes && !pr.Confirm("Apply these changes?", true) {
			fmt.Fprintln(out, co.yellow.Render("Aborted. Nothing was changed."))
			return nil
		}
	}

	backups := filepath.Join(s.CommonDir, "info", "devex", "sync-backups", time.Now().Format("2006-01-02-150405"))
	ops, err := wtsync.Resolve(plan, pr)
	if err != nil {
		fmt.Fprintln(errw, ce.yellow.Render(err.Error()))
		return fmt.Errorf("%w: %w", ErrReported, err)
	}
	used, err := wtsync.Apply(ops, backups)
	reportBackups := func(w io.Writer) {
		if used != "" {
			fmt.Fprintln(w, "Backups stored in: "+used)
		}
	}
	if err != nil {
		fmt.Fprintln(errw, ce.red.Render(err.Error()))
		reportBackups(errw)
		fmt.Fprintln(errw, "The sync manifest was not updated.")
		return ErrReported
	}
	wtsync.PruneEmptyDirs(append([]string{s.Main}, s.Others...), s.CopyPaths)
	fmt.Fprintln(out, co.green.Render("Rebuilding sync manifest..."))
	if err := s.RebuildManifest(); err != nil {
		return fail(err)
	}
	fmt.Fprintln(out, "\n"+co.green.Render("✓ Synchronization complete!"))
	reportBackups(out)
	return nil
}
