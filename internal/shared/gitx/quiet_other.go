//go:build !unix

package gitx

import "os/exec"

func detach(*exec.Cmd) {}
