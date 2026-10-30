// Package cli holds the command trees for the dx and git-wt binaries.
package cli

import (
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// Version is set by cmd/dx from the build-time version.
var Version = "dev"

var errExit1 = errors.New("exit 1")

var dxCmds = map[string][]*cobra.Command{}

// dxGroupRun lets a group override its bare/unknown-subcommand behaviour (default: print help).
var dxGroupRun = map[string]func(*cobra.Command, []string) error{}

// registerDx lets later tasks attach subcommands to the skills, nb or ctx group.
func registerDx(group string, cmd *cobra.Command) { dxCmds[group] = append(dxCmds[group], cmd) }

func NewDxRoot() *cobra.Command {
	root := &cobra.Command{Use: "dx", Short: "DevEx Manager", Version: Version, Args: cobra.ArbitraryArgs}
	root.RunE = func(c *cobra.Command, _ []string) error {
		c.Help()
		return errExit1
	}
	for _, g := range []struct{ name, short string }{
		{"skills", "Sync AI-agent skill folders against a master directory"},
		{"nb", "Jupyter notebook hygiene utilities"},
		{"ctx", "Per-branch scratchpad notes"},
	} {
		grp := &cobra.Command{Use: g.name, Short: g.short, Args: cobra.ArbitraryArgs,
			RunE: func(c *cobra.Command, _ []string) error { return c.Help() }}
		if f := dxGroupRun[g.name]; f != nil {
			grp.RunE = f
		}
		grp.DisableFlagParsing = g.name == "ctx" // ctx takes raw args: "dx ctx -x" adds a task
		for _, c := range dxCmds[g.name] {
			resetFlags(c)
			grp.AddCommand(c)
		}
		root.AddCommand(grp)
	}
	return root
}

// Run executes the tree selected by argv0 and returns the process exit code.
func Run(argv0 string, args []string, in *os.File, out, errw io.Writer) int {
	root := NewDxRoot()
	if filepath.Base(argv0) == "git-wt" {
		root = NewGitWtRoot()
	}
	root.SilenceUsage, root.SilenceErrors = true, true
	root.SetArgs(args)
	root.SetIn(in)
	root.SetOut(out)
	root.SetErr(errw)
	if err := root.Execute(); err != nil {
		var ec exitCode
		if errors.As(err, &ec) {
			return int(ec)
		}
		if err != errExit1 {
			io.WriteString(errw, "Error: "+err.Error()+"\n")
		}
		return 1
	}
	return 0
}

func Main(argv0 string, args []string) int {
	return Run(argv0, args, os.Stdin, os.Stdout, os.Stderr)
}
