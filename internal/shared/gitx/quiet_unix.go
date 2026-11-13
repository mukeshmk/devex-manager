//go:build unix

package gitx

import (
	"os/exec"
	"syscall"
)

// A new session means ssh cannot open /dev/tty to ask for a passphrase.
func detach(c *exec.Cmd) { c.SysProcAttr = &syscall.SysProcAttr{Setsid: true} }
