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
	"github.com/mukeshmk/devex-manager/internal/wtapp"
)

func init() {
	cmd := &cobra.Command{
		Use:     "status",
		Aliases: []string{"list"},
		Short:   "Rich status overview (alias: list)",
		Args:    cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			return runStatus(c, c.OutOrStdout(), c.ErrOrStderr())
		},
	}
	registerWt(cmd)
}

func runStatus(c *cobra.Command, out, errw io.Writer) error {
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
	if interactive(c) {
		self, err := os.Executable()
		if err != nil {
			return fail(err)
		}
		if err := wtapp.Run(wtapp.New(repo, cfg, self)); err != nil {
			return fail(err)
		}
		return nil
	}
	rows, err := wt.Collect(repo, cfg)
	if err != nil {
		return fail(err)
	}
	f, _ := out.(*os.File)
	fmt.Fprint(out, wt.RenderTable(rows, ui.Width(f)))
	return nil
}
