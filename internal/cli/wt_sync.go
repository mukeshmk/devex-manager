package cli

import (
	"io"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/mukeshmk/devex-manager/internal/gitwt/flows"

	"github.com/mukeshmk/devex-manager/internal/shared/ui"
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
	e, err := wtEnv(in, out, errw)
	if err != nil {
		return err
	}
	if e.TTY { // off a TTY the flow never asks, so no reader is built over a possibly nil stdin
		e.Prompter = ui.NewPrompter(in, out)
	}
	return exit1(flows.Sync(e, flows.SyncOpts{Check: check, DryRun: dryRun, Yes: yes}))
}
