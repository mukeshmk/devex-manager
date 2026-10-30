// Package gitx wraps the git CLI for the worktree tools.
package gitx

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Command describes one finished git process, as reported to the hook.
type Command struct {
	Dir      string
	Args     []string
	Err      error
	Duration time.Duration
}

var (
	hookMu sync.RWMutex
	hook   func(Command)
)

// SetCommandHook registers fn to run after every git process exits, on the
// goroutine that ran it. nil disables the hook.
func SetCommandHook(fn func(Command)) {
	hookMu.Lock()
	hook = fn
	hookMu.Unlock()
}

func notify(c Command) {
	hookMu.RLock()
	fn := hook
	hookMu.RUnlock()
	if fn != nil {
		fn(c)
	}
}

// Run executes git in dir and returns trimmed stdout; errors include stderr.
func Run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	start := time.Now()
	err := cmd.Run()
	notify(Command{Dir: dir, Args: args, Err: err, Duration: time.Since(start)})
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(errb.String()))
	}
	return strings.TrimSpace(out.String()), nil
}

// Repo identifies a repository by its common dir and logical root.
type Repo struct{ CommonDir, Root string }

func abs(cwd, p string) string {
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Join(cwd, p)
}

// Open resolves repo root like git-wt-sync: bare -> git dir (parent if .git), else toplevel.
func Open(cwd string) (*Repo, error) {
	gd, err := Run(cwd, "rev-parse", "--git-dir")
	if err != nil {
		return nil, fmt.Errorf("Error: Not in a git repository")
	}
	cd, err := Run(cwd, "rev-parse", "--git-common-dir")
	if err != nil {
		return nil, err
	}
	gd, cd = abs(cwd, gd), abs(cwd, cd)
	trim := func(p string) string {
		if filepath.Base(p) == ".git" {
			return filepath.Dir(p)
		}
		return p
	}
	r := &Repo{CommonDir: cd}
	if b, _ := Run(cwd, "rev-parse", "--is-bare-repository"); b == "true" {
		r.Root = trim(gd)
	} else if b, _ := Run(cwd, "--git-dir="+cd, "config", "--bool", "core.bare"); b == "true" {
		r.Root = trim(cd)
	} else if r.Root, err = Run(cwd, "rev-parse", "--show-toplevel"); err != nil {
		return nil, err
	}
	return r, nil
}

// MergedBranches returns local branches merged into base.
func (r *Repo) MergedBranches(base string) (map[string]bool, error) {
	out, err := Run(r.CommonDir, "for-each-ref", "--merged="+base, "--format=%(refname:short)", "refs/heads")
	if err != nil {
		return nil, err
	}
	return lineSet(out), nil
}

// RemoteHeads lists branch names on remote with a single ls-remote call.
func (r *Repo) RemoteHeads(remote string) (map[string]bool, error) {
	out, err := Run(r.CommonDir, "ls-remote", "--heads", remote)
	if err != nil {
		return nil, err
	}
	m := map[string]bool{}
	for _, l := range strings.Split(out, "\n") {
		if _, ref, ok := strings.Cut(l, "\t"); ok {
			m[strings.TrimPrefix(ref, "refs/heads/")] = true
		}
	}
	return m, nil
}

// Upstream returns the configured remote for branch.
func (r *Repo) Upstream(branch string) (string, bool) {
	out, err := Run(r.CommonDir, "config", "--get", "branch."+branch+".remote")
	return out, err == nil && out != ""
}

// AheadBehind compares branch to its upstream; hasUpstream is false if none.
func AheadBehind(dir, branch string) (ahead, behind int, hasUpstream bool, err error) {
	if _, err = Run(dir, "rev-parse", "--verify", "-q", "refs/heads/"+branch); err != nil {
		return 0, 0, false, err
	}
	if _, e := Run(dir, "config", "--get", "branch."+branch+".merge"); e != nil {
		return 0, 0, false, nil // key unset: no upstream configured
	}
	out, err := Run(dir, "rev-list", "--left-right", "--count", branch+"..."+branch+"@{upstream}")
	if err != nil {
		return 0, 0, true, err
	}
	f := strings.Fields(out)
	if len(f) != 2 {
		return 0, 0, true, fmt.Errorf("unexpected rev-list output %q", out)
	}
	if ahead, err = strconv.Atoi(f[0]); err != nil {
		return 0, 0, true, err
	}
	if behind, err = strconv.Atoi(f[1]); err != nil {
		return 0, 0, true, err
	}
	return ahead, behind, true, nil
}

// ChangedFiles returns `status --porcelain` lines.
func ChangedFiles(dir string, extra ...string) ([]string, error) {
	// Not Run(): its TrimSpace would eat the leading space of " M file".
	args := append([]string{"status", "--porcelain"}, extra...)
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var errb bytes.Buffer
	cmd.Stderr = &errb
	start := time.Now()
	b, err := cmd.Output()
	notify(Command{Dir: dir, Args: args, Err: err, Duration: time.Since(start)})
	if err != nil {
		return nil, fmt.Errorf("git status: %w: %s", err, strings.TrimSpace(errb.String()))
	}
	var files []string
	for _, l := range strings.Split(string(b), "\n") {
		if l != "" {
			files = append(files, l)
		}
	}
	return files, nil
}

// UnpushedCount counts commits on branch not reachable from any remote ref.
func UnpushedCount(dir, branch string) (int, error) {
	out, err := Run(dir, "rev-list", "--count", branch, "--not", "--remotes")
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(out)
}

func lineSet(s string) map[string]bool {
	m := map[string]bool{}
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			m[l] = true
		}
	}
	return m
}
