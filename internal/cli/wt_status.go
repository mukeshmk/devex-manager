package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/mukeshmk/devex-manager/internal/config"
	"github.com/mukeshmk/devex-manager/internal/gitx"
	"github.com/mukeshmk/devex-manager/internal/ui"
	"github.com/mukeshmk/devex-manager/internal/wt"
)

func init() {
	cmd := &cobra.Command{
		Use:     "status",
		Aliases: []string{"list"},
		Short:   "Rich status overview (alias: list)",
		Args:    cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			return runStatus(c.OutOrStdout(), c.ErrOrStderr())
		},
	}
	registerWt(cmd)
}

func runStatus(out, errw io.Writer) error {
	fail := func(err error) error {
		fmt.Fprintln(errw, colours(errw).red.Render(err.Error()))
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
	rows, err := wt.Collect(repo, cfg)
	if err != nil {
		return fail(err)
	}
	width := 100
	if f, ok := out.(*os.File); ok && ui.IsTTY(f) {
		width = ui.Width(f) // Task 12 swaps in the full-screen app here
	}
	fmt.Fprint(out, wt.RenderTable(rows, width))
	return nil
}
