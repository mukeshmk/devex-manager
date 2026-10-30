package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/mukeshmk/devex-manager/internal/gitwt/flows"
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
	ce := colours(errw)
	usage := func(msg string) error {
		fmt.Fprintln(errw, ce.red.Render(msg))
		fmt.Fprintln(errw, rmUsage)
		return errExit1
	}
	switch {
	case len(args) == 0:
		return usage("Error: Target (folder or branch name) is required")
	case len(args) > 1:
		return usage("Error: Too many arguments specified")
	}
	e, err := wtEnv(nil, out, errw)
	if err != nil {
		return err
	}
	return exit1(flows.Remove(e, args[0], force))
}
