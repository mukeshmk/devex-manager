package cli

import (
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/mukeshmk/devex-manager/internal/gitwt/flows"

	"github.com/mukeshmk/devex-manager/internal/shared/ui"
)

func init() {
	var yes bool
	cmd := &cobra.Command{
		Use:   "clean [--yes|-y]",
		Short: "Interactively remove stale worktrees",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			in, _ := c.InOrStdin().(*os.File)
			return runClean(in, c.OutOrStdout(), c.ErrOrStderr(), yes)
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "remove the preselected (safe) worktrees without asking")
	registerWt(cmd)
}

func runClean(in *os.File, out, errw io.Writer, yes bool) error {
	e, err := wtEnv(in, out, errw)
	if err != nil {
		return err
	}
	e.Prompter = ui.NewPrompter(in, out) // LinePrompter off a TTY, so `echo y |` works
	return exit1(flows.Clean(e, yes))
}
