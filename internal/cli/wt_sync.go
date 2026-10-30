package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/mukeshmk/devex-manager/internal/config"
	"github.com/mukeshmk/devex-manager/internal/gitx"
	"github.com/mukeshmk/devex-manager/internal/ui"
	"github.com/mukeshmk/devex-manager/internal/wtsync"
)

func init() {
	var check, dryRun, yes bool
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Synchronize configuration copies",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			in, _ := c.InOrStdin().(*os.File)
			return runSync(in, c.OutOrStdout(), c.ErrOrStderr(), check, dryRun, yes)
		},
	}
	cmd.Flags().BoolVarP(&check, "check", "c", false, "only report files that are out of sync")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show the plan without changing anything")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation (conflicts are still asked on a TTY)")
	registerWt(cmd)
}

type palette struct{ red, green, yellow, blue lipgloss.Style }

func colours(w io.Writer) palette {
	r := lipgloss.NewRenderer(w) // plain text unless w is a colour terminal
	st := func(c string) lipgloss.Style { return r.NewStyle().Foreground(lipgloss.Color(c)) }
	return palette{st("1"), st("2"), st("11"), st("4")}
}

func runSync(in *os.File, out, errw io.Writer, check, dryRun, yes bool) error {
	co, ce := colours(out), colours(errw)
	fail := func(err error) error {
		fmt.Fprintln(errw, ce.red.Render(err.Error()))
		return errExit1
	}
	cwd, err := os.Getwd()
	if err != nil {
		return fail(err)
	}
	repo, err := gitx.Open(cwd)
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
		return errExit1
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
			return errExit1
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

	tty := ui.IsTTY(in)
	var pr ui.Prompter = ui.DefaultPrompter{}
	if tty {
		pr = ui.NewPrompter(in, out)
		if len(plan.Steps) > 0 && !yes && !pr.Confirm("Apply these changes?", true) {
			fmt.Fprintln(out, co.yellow.Render("Aborted. Nothing was changed."))
			return nil
		}
	}

	backups := filepath.Join(s.CommonDir, "info", "devex", "sync-backups", time.Now().Format("2006-01-02-150405"))
	used, err := wtsync.Apply(wtsync.Resolve(plan, pr), backups)
	reportBackups := func(w io.Writer) {
		if used != "" {
			fmt.Fprintln(w, "Backups stored in: "+used)
		}
	}
	if err != nil {
		fmt.Fprintln(errw, ce.red.Render(err.Error()))
		reportBackups(errw)
		fmt.Fprintln(errw, "The sync manifest was not updated.")
		return errExit1
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
