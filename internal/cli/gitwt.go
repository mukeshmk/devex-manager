package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

const gitWtHelp = `DevEx Manager (git wt)
Usage: git wt <command> [arguments]

Available commands:
  clone   - Initialize bare repo & main worktree
  add     - Create/checkout worktree (by branch prefix)
  rm      - Delete worktree folder & local branch (supports: --force, -f)
  clean   - Interactively remove stale worktrees
  status  - Rich status overview (alias: list)
  sync    - Synchronize configuration copies

Native Git Commands Pass-through:
  lock, move, prune, repair, unlock
`

var wtCmds []*cobra.Command

// registerWt lets later tasks attach git-wt subcommands.
func registerWt(cmd *cobra.Command) { wtCmds = append(wtCmds, cmd) }

// Passthrough maps args to a native `git worktree` invocation, mirroring the Bash router.
func Passthrough(args []string) (gitArgs []string, ok bool) {
	if len(args) == 0 {
		return nil, false
	}
	switch args[0] {
	case "lock", "move", "prune", "repair", "unlock":
	case "list":
		if len(args) < 2 {
			return nil, false
		}
	default:
		return nil, false
	}
	return append([]string{"worktree"}, args...), true
}

func NewGitWtRoot() *cobra.Command {
	root := &cobra.Command{Use: "git-wt", Args: cobra.ArbitraryArgs}
	root.SetHelpFunc(func(c *cobra.Command, _ []string) {
		if c == root {
			fmt.Fprint(c.OutOrStdout(), gitWtHelp)
			return
		}
		c.Usage()
	})
	root.RunE = func(c *cobra.Command, _ []string) error {
		fmt.Fprint(c.OutOrStdout(), gitWtHelp)
		return errExit1
	}
	for _, c := range wtCmds {
		root.AddCommand(c)
	}
	return root
}
