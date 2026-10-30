package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/mukeshmk/devex-manager/internal/wt"
)

func init() {
	registerWt(&cobra.Command{
		Use:   "clone <url>",
		Short: "Initialize bare repo & main worktree",
		Args:  cobra.ArbitraryArgs,
		RunE: func(c *cobra.Command, args []string) error {
			errw := c.ErrOrStderr()
			if len(args) != 1 {
				msg := "Error: Git repository URL is required"
				if len(args) > 1 {
					msg = "Error: Too many arguments specified"
				}
				fmt.Fprintln(errw, colours(errw).red.Render(msg))
				fmt.Fprintln(errw, "Usage: git wt clone <url>")
				return errExit1
			}
			cwd, err := os.Getwd()
			if err == nil {
				err = wt.Clone(args[0], cwd, c.OutOrStdout())
			}
			if err != nil {
				fmt.Fprintln(errw, colours(errw).red.Render(err.Error()))
				return errExit1
			}
			return nil
		},
	})
}
