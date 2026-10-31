package gitx

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"time"
)

var quiet atomic.Bool

// SetQuiet makes every git run non-interactive (no prompt, no controlling tty); the full-screen app owns the terminal.
func SetQuiet(on bool) { quiet.Store(on) }

// command builds a git exec.Cmd; with q it never prompts and has no controlling tty.
func command(dir string, q bool, args ...string) *exec.Cmd {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if q {
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
		detach(cmd)
	}
	return cmd
}

// RunQuiet is Run for callers that own the terminal (the full-screen app): git
// never prompts and has no controlling tty. It returns stdout and stderr combined.
func RunQuiet(dir string, args ...string) (string, error) {
	cmd := command(dir, true, args...)
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	start := time.Now()
	err := cmd.Run()
	notify(Command{Dir: dir, Args: args, Err: err, Duration: time.Since(start)})
	out := strings.TrimSpace(buf.String())
	if err != nil {
		return out, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, out)
	}
	return out, nil
}
