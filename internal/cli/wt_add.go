package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/mukeshmk/devex-manager/internal/gitwt/worktree"
	"github.com/mukeshmk/devex-manager/internal/shared/config"
	"github.com/mukeshmk/devex-manager/internal/shared/gitx"
	"github.com/mukeshmk/devex-manager/internal/shared/ui"
)

func init() {
	registerWt(&cobra.Command{
		Use:   "add [branch] [folder]",
		Short: "Create/checkout worktree (by branch prefix)",
		Args:  cobra.MaximumNArgs(2),
		RunE: func(c *cobra.Command, args []string) error {
			in, _ := c.InOrStdin().(*os.File)
			return runAdd(in, c.OutOrStdout(), c.ErrOrStderr(), args)
		},
	})
}

func runAdd(in *os.File, out, errw io.Writer, args []string) error {
	co, ce := colours(out), colours(errw)
	fail := func(lines ...string) error {
		fmt.Fprintln(errw, ce.red.Render(lines[0]))
		for _, l := range lines[1:] {
			fmt.Fprintln(errw, l)
		}
		return errExit1
	}
	cwd, err := os.Getwd()
	if err != nil {
		return fail(err.Error())
	}
	var branch, folder string
	if len(args) > 0 {
		branch = args[0]
	}
	if len(args) > 1 {
		folder = args[1]
	}
	if branch == "" {
		if !ui.IsTTY(in) {
			return fail("Error: Branch name is required", "Usage: git wt-add <branch-name> [custom-folder-name]")
		}
		if branch, err = pickRemoteBranch(cwd, out); err != nil {
			return fail(err.Error())
		}
		if branch == "" {
			fmt.Fprintln(out, co.yellow.Render("No branch selected. Exiting."))
			return nil
		}
		fmt.Fprintln(out, co.blue.Render("Selected branch: ")+branch)
	}
	repo, err := gitx.Open(cwd)
	if err != nil {
		return fail(err.Error())
	}
	cfg, err := config.LoadRepo(repo.Root)
	if err != nil {
		return fail(err.Error())
	}
	if _, err := wt.Add(repo, cfg, branch, folder, out); err != nil {
		return fail(err.Error())
	}
	return nil
}

// pickRemoteBranch returns "" when the user cancels or nothing is left to pick.
func pickRemoteBranch(cwd string, out io.Writer) (string, error) {
	repo, err := gitx.Open(cwd)
	if err != nil {
		return "", err
	}
	fmt.Fprintln(out, "No branch specified. Searching for remote branches...")
	raw, err := gitx.Run(repo.CommonDir, "branch", "-r")
	if err != nil {
		return "", err
	}
	wts, err := repo.Worktrees()
	if err != nil {
		return "", err
	}
	choices := wt.RemoteBranchChoices(raw, wts)
	if len(choices) == 0 {
		return "", nil
	}
	var sel string
	err = huh.NewSelect[string]().Title("Select a remote branch to create a worktree for:").
		Options(huh.NewOptions(choices...)...).Filtering(true).Height(12).Value(&sel).Run()
	if err == huh.ErrUserAborted {
		return "", nil
	}
	return sel, err
}
