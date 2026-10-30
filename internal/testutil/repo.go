// Package testutil provides an isolated temp git repo fixture.
package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Fixture is a `git wt clone`-style layout: Root/.git (bare) + Root/main.
type Fixture struct {
	Tmp, Origin, Root string
	env               []string
}

var isolated = []string{
	"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
	"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
	"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com",
}

// Env returns the isolated git environment.
func (f *Fixture) Env() []string { return append(os.Environ(), isolated...) }

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// NewRepo builds the fixture and applies the isolated env via t.Setenv.
func NewRepo(t *testing.T) *Fixture {
	t.Helper()
	for _, kv := range isolated {
		for i := 0; i < len(kv); i++ {
			if kv[i] == '=' {
				t.Setenv(kv[:i], kv[i+1:])
				break
			}
		}
	}
	tmp, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := &Fixture{Tmp: tmp, Origin: filepath.Join(tmp, "origin.git"), Root: filepath.Join(tmp, "repo")}
	seed := filepath.Join(tmp, "seed")
	git(t, tmp, "init", "-q", "--bare", "-b", "main", f.Origin)
	git(t, tmp, "init", "-q", "-b", "main", seed)
	git(t, seed, "commit", "-q", "--allow-empty", "-m", "init")
	git(t, seed, "push", "-q", f.Origin, "main")
	if err := os.MkdirAll(f.Root, 0o755); err != nil {
		t.Fatal(err)
	}
	bare := filepath.Join(f.Root, ".git")
	git(t, tmp, "clone", "-q", "--bare", f.Origin, bare)
	git(t, bare, "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*")
	git(t, bare, "fetch", "-q", "origin")
	git(t, bare, "worktree", "add", "-q", filepath.Join(f.Root, "main"), "main")
	return f
}

// AddWorktree creates branch (from main) in Root/folder and returns its absolute path.
func (f *Fixture) AddWorktree(t *testing.T, folder, branch string) string {
	t.Helper()
	p := filepath.Join(f.Root, folder)
	git(t, filepath.Join(f.Root, ".git"), "worktree", "add", "-q", "-b", branch, p, "main")
	return p
}
