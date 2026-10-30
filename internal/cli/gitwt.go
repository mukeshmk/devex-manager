package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const gitWtHelp = `DevEx Manager (git wt)
Usage: git wt <command> [arguments]

Available commands:
  clone   - Initialize bare repo & main worktree
  add     - Create/checkout worktree (by branch prefix)
  rm      - Delete worktree folder & local branch
            -f, --force   skip the sync and unsaved-work checks
  clean   - Interactively remove stale worktrees
            -y, --yes     remove the preselected (safe) ones without asking
  status  - Rich status overview (alias: list)
  sync    - Synchronize configuration copies
            -c, --check   only report files that are out of sync
            --dry-run     show the plan without changing anything
            -y, --yes     skip the confirmation

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
	root := &cobra.Command{Use: "git-wt", Version: Version, Args: cobra.ArbitraryArgs}
	root.SetHelpFunc(func(c *cobra.Command, _ []string) {
		if c == root {
			fmt.Fprint(c.OutOrStdout(), gitWtHelp)
			return
		}
		c.Usage()
	})
	root.RunE = func(c *cobra.Command, args []string) error {
		if len(args) == 0 && interactive(c) { // bare `git wt` on a terminal opens the app
			return runStatus(c, c.OutOrStdout(), c.ErrOrStderr())
		}
		fmt.Fprint(c.OutOrStdout(), gitWtHelp)
		return errExit1
	}
	for _, c := range wtCmds {
		resetFlags(c)
		root.AddCommand(c)
	}
	return root
}

// resetFlags restores defaults: registered commands are package-level, so flag values set by
// one Run would otherwise leak into the next (tests run many in one process).
func resetFlags(c *cobra.Command) {
	c.Flags().VisitAll(func(f *pflag.Flag) {
		// shortcut: slice flags reset to empty, not to a non-empty default; none exist yet
		if sv, ok := f.Value.(pflag.SliceValue); ok {
			sv.Replace(nil)
		} else {
			f.Value.Set(f.DefValue)
		}
		f.Changed = false
	})
}
