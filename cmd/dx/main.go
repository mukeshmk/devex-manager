package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/mukeshmk/devex-manager/internal/cli"
)

var version = "dev"

func main() {
	cli.Version = version
	args := os.Args[1:]
	if filepath.Base(os.Args[0]) == "git-wt" {
		if g, ok := cli.Passthrough(args); ok {
			git, err := exec.LookPath("git")
			if err == nil {
				err = syscall.Exec(git, append([]string{"git"}, g...), os.Environ())
			}
			fmt.Fprintln(os.Stderr, "git-wt:", err)
			os.Exit(1)
		}
	}
	os.Exit(cli.Main(os.Args[0], args))
}
