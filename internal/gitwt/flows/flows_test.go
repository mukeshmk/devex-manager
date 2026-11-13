package flows

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mukeshmk/devex-manager/internal/gitwt/sync"
	"github.com/mukeshmk/devex-manager/internal/shared/gitx"
	"github.com/mukeshmk/devex-manager/internal/shared/testutil"
)

// script is a ui.Prompter that records questions and answers with its defaults.
type script struct {
	asked   []string
	options [][]string
	pre     []bool
	labels  []string
	abort   bool
	pick    int
}

func (s *script) Confirm(q string, def bool) bool { s.asked = append(s.asked, q); return def }
func (s *script) Choose(q string, o []string, def int) int {
	s.asked = append(s.asked, q)
	s.options = append(s.options, o)
	return s.pick
}
func (s *script) MultiSelect(q string, o []string, pre []bool) []bool {
	s.asked = append(s.asked, q)
	s.pre, s.labels = pre, o
	return make([]bool, len(o))
}
func (s *script) Aborted() bool { return s.abort }

func write(t *testing.T, p, body string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// conflictRepo has a copy path whose file differs in main and feat.
func conflictRepo(t *testing.T) (main, feat string) {
	t.Helper()
	f := testutil.NewRepo(t)
	main = filepath.Join(f.Root, "main")
	feat = f.AddWorktree(t, "feat", "feat")
	write(t, filepath.Join(f.Root, ".devex.conf"), "[copies]\npaths = .vscode\n")
	write(t, filepath.Join(main, ".vscode", "a.json"), "main")
	write(t, filepath.Join(feat, ".vscode", "a.json"), "feat")
	return main, feat
}

func env(main string, p *script) (Env, *bytes.Buffer) {
	var b bytes.Buffer
	return Env{Cwd: main, Out: &b, Err: &b, Prompter: p, TTY: true}, &b
}

func TestSyncFlowUsesEnvPrompter(t *testing.T) {
	main, _ := conflictRepo(t)
	p := &script{}
	e, _ := env(main, p)
	if err := Sync(e, SyncOpts{Yes: true}); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, o := range p.options {
		found = found || strings.Contains(strings.Join(o, "|"), "Resolve Later")
	}
	if !found {
		t.Fatalf("no Resolve Later choice: %v", p.options)
	}
}

func TestSyncFlowAbort(t *testing.T) {
	main, feat := conflictRepo(t)
	p := &script{abort: true}
	e, b := env(main, p)
	err := Sync(e, SyncOpts{Yes: true})
	if !errors.Is(err, wtsync.ErrAborted) || !errors.Is(err, ErrReported) {
		t.Fatalf("err=%v", err)
	}
	for path, want := range map[string]string{filepath.Join(main, ".vscode", "a.json"): "main", filepath.Join(feat, ".vscode", "a.json"): "feat"} {
		if got, _ := os.ReadFile(path); string(got) != want {
			t.Fatalf("%s changed to %q\n%s", path, got, b)
		}
	}
}

func TestCleanFlowPreselectsSafe(t *testing.T) {
	f := testutil.NewRepo(t)
	f.AddWorktree(t, "gone", "gone") // merged into base, clean: safe
	dirty := f.AddWorktree(t, "dirty", "dirty")
	write(t, filepath.Join(dirty, "x.txt"), "unsaved")
	main := filepath.Join(f.Root, "main")
	p := &script{}
	e, b := env(main, p)
	if err := Clean(e, false); err != nil {
		t.Fatalf("%v\n%s", err, b)
	}
	if len(p.pre) == 0 {
		t.Fatalf("no MultiSelect asked\n%s", b)
	}
	for i, l := range p.labels {
		if want := strings.HasPrefix(l, "gone:"); p.pre[i] != want {
			t.Fatalf("pre[%d]=%v for %q", i, p.pre[i], l)
		}
	}
}

func TestRemoveFlowRefusesDirty(t *testing.T) {
	f := testutil.NewRepo(t)
	d := f.AddWorktree(t, "dirty", "dirty")
	write(t, filepath.Join(d, "x.txt"), "unsaved")
	e, b := env(filepath.Join(f.Root, "main"), &script{})
	err := Remove(e, "dirty", false, DirtyRefuse)
	if !errors.Is(err, ErrReported) || !strings.Contains(b.String(), "Uncommitted changes") {
		t.Fatalf("err=%v out=%s", err, b)
	}
	if _, serr := os.Stat(d); serr != nil {
		t.Fatal("worktree removed")
	}
}

func addEnv(dir string) (Env, *bytes.Buffer, *bytes.Buffer) {
	var o, e bytes.Buffer
	return Env{Cwd: dir, Out: &o, Err: &e}, &o, &e
}

func TestAddFlowCreatesBranchFromBase(t *testing.T) {
	f := testutil.NewRepo(t)
	main := filepath.Join(f.Root, "main")
	write(t, filepath.Join(main, ".vscode", "a.json"), "x")
	e, _, errb := addEnv(main)
	if err := Add(e, "feat-x", ""); err != nil {
		t.Fatalf("%v: %s", err, errb)
	}
	p := filepath.Join(f.Root, "feat-x")
	if b, _ := gitx.Run(p, "branch", "--show-current"); b != "feat-x" {
		t.Fatalf("branch %q", b)
	}
	if l, err := os.Readlink(filepath.Join(p, ".vscode")); err != nil || l != filepath.Join(main, ".vscode") {
		t.Fatalf("symlink %q %v", l, err)
	}
}

func TestAddFlowExistingRemoteBranch(t *testing.T) {
	f := testutil.NewRepo(t)
	seed := filepath.Join(f.Tmp, "seed")
	gitx.Run(seed, "push", "-q", f.Origin, "main:refs/heads/r1")
	if _, err := gitx.Run(f.Root, "fetch", "-q", "origin"); err != nil {
		t.Fatal(err)
	}
	e, _, errb := addEnv(filepath.Join(f.Root, "main"))
	if err := Add(e, "r1", ""); err != nil {
		t.Fatalf("%v: %s", err, errb)
	}
	if b, _ := gitx.Run(filepath.Join(f.Root, "r1"), "branch", "--show-current"); b != "r1" {
		t.Fatalf("branch %q", b)
	}
}

func TestAddFlowErrorReported(t *testing.T) {
	e, _, errb := addEnv(t.TempDir())
	if err := Add(e, "x", ""); !errors.Is(err, ErrReported) || !strings.Contains(errb.String(), "Not in a git repository") {
		t.Fatalf("%v %q", err, errb)
	}
}

func TestFetchFlowNoRemoteFails(t *testing.T) {
	f := testutil.NewRepo(t)
	gitx.Run(f.Root, "remote", "set-url", "origin", filepath.Join(f.Tmp, "nope.git"))
	e, _, errb := addEnv(filepath.Join(f.Root, "main"))
	if err := Fetch(e); !errors.Is(err, ErrReported) || errb.Len() == 0 {
		t.Fatalf("%v %q", err, errb)
	}
}

func TestFetchFlowPrunesAndFetches(t *testing.T) {
	f := testutil.NewRepo(t)
	seed := filepath.Join(f.Tmp, "seed")
	gitx.Run(seed, "push", "-q", f.Origin, "main:refs/heads/new1", "main:refs/heads/gone")
	e, _, errb := addEnv(filepath.Join(f.Root, "main"))
	if err := Fetch(e); err != nil {
		t.Fatalf("%v: %s", err, errb)
	}
	has := func() map[string]bool {
		bs, err := RemoteBranches(e.Cwd)
		if err != nil {
			t.Fatal(err)
		}
		m := map[string]bool{}
		for _, b := range bs {
			m[b] = true
		}
		return m
	}
	if m := has(); !m["new1"] || !m["gone"] {
		t.Fatalf("%v", m)
	}
	gitx.Run(f.Origin, "branch", "-q", "-D", "gone")
	if err := Fetch(e); err != nil {
		t.Fatal(err)
	}
	if m := has(); !m["new1"] || m["gone"] {
		t.Fatalf("not pruned: %v", m)
	}
}

func TestFetchFlowHeadlineIsGitError(t *testing.T) {
	f := testutil.NewRepo(t)
	gitx.Run(f.Root, "remote", "add", "bad", filepath.Join(f.Tmp, "nope.git"))
	e, _, errb := addEnv(filepath.Join(f.Root, "main"))
	if err := Fetch(e); !errors.Is(err, ErrReported) {
		t.Fatal(err)
	}
	first, _, _ := strings.Cut(errb.String(), "\n")
	if !strings.HasPrefix(first, "fatal:") && !strings.HasPrefix(first, "error:") {
		t.Fatalf("headline %q in:\n%s", first, errb)
	}
}

func TestRemoveFlowStashesDirty(t *testing.T) {
	f := testutil.NewRepo(t)
	d := f.AddWorktree(t, "dirty", "dirty")
	write(t, filepath.Join(d, "a.txt"), "staged")
	gitx.Run(d, "add", "a.txt")
	write(t, filepath.Join(d, "x.txt"), "unsaved")
	e, b := env(filepath.Join(f.Root, "main"), &script{})
	if err := Remove(e, "dirty", false, DirtyStash); err != nil {
		t.Fatalf("%v\n%s", err, b)
	}
	if _, err := os.Stat(d); !os.IsNotExist(err) {
		t.Fatal("worktree not removed")
	}
	git := filepath.Join(f.Root, "main") // stash needs a work tree; the stash itself is shared
	if l, _ := gitx.Run(git, "stash", "list"); !strings.Contains(l, "dirty") {
		t.Fatalf("no stash for dirty: %q", l)
	}
	if s, err := gitx.Run(git, "show", "stash@{0}^3:x.txt"); err != nil || s != "unsaved" {
		t.Fatalf("untracked file not stashed: %q %v", s, err)
	}
	if s, _ := gitx.Run(git, "show", "stash@{0}:a.txt"); s != "staged" {
		t.Fatalf("tracked change not stashed: %q", s)
	}
}

func TestRemoveFlowDiscardsDirty(t *testing.T) {
	f := testutil.NewRepo(t)
	d := f.AddWorktree(t, "dirty", "dirty")
	write(t, filepath.Join(d, "x.txt"), "unsaved")
	e, b := env(filepath.Join(f.Root, "main"), &script{})
	if err := Remove(e, "dirty", false, DirtyDiscard); err != nil {
		t.Fatalf("%v\n%s", err, b)
	}
	if _, err := os.Stat(d); !os.IsNotExist(err) {
		t.Fatal("worktree not removed")
	}
	if l, _ := gitx.Run(filepath.Join(f.Root, "main"), "stash", "list"); l != "" {
		t.Fatalf("discard stashed: %q", l)
	}
}

func TestRemoveFlowStashStillRefusesUnpushed(t *testing.T) {
	f := testutil.NewRepo(t)
	d := f.AddWorktree(t, "dirty", "dirty")
	write(t, filepath.Join(d, "c.txt"), "c")
	gitx.Run(d, "add", "c.txt")
	gitx.Run(d, "commit", "-qm", "local only")
	write(t, filepath.Join(d, "x.txt"), "unsaved")
	for _, how := range []Dirty{DirtyStash, DirtyDiscard} {
		e, b := env(filepath.Join(f.Root, "main"), &script{})
		if err := Remove(e, "dirty", false, how); !errors.Is(err, ErrReported) || !strings.Contains(b.String(), "not on any remote") {
			t.Fatalf("%v: err=%v out=%s", how, err, b)
		}
		if _, err := os.Stat(filepath.Join(d, "x.txt")); err != nil {
			t.Fatalf("%v: work lost", how)
		}
	}
	if l, _ := gitx.Run(filepath.Join(f.Root, "main"), "stash", "list"); l != "" {
		t.Fatalf("stashed despite refusal: %q", l)
	}
}

func TestRemoveFlowStashSkipsCopyPaths(t *testing.T) {
	f := testutil.NewRepo(t)
	main := filepath.Join(f.Root, "main")
	d := f.AddWorktree(t, "dirty", "dirty")
	write(t, filepath.Join(f.Root, ".devex.conf"), "[copies]\npaths = .vscode\n")
	write(t, filepath.Join(main, ".vscode", "a.json"), "same")
	e, b := env(main, &script{})
	if err := Sync(e, SyncOpts{Yes: true}); err != nil { // copies .vscode into dirty
		t.Fatalf("%v\n%s", err, b)
	}
	write(t, filepath.Join(d, "x.txt"), "unsaved")
	if err := Remove(e, "dirty", false, DirtyStash); err != nil {
		t.Fatalf("%v\n%s", err, b)
	}
	if files, _ := gitx.Run(main, "show", "--name-only", "--format=", "stash@{0}^3"); files != "x.txt" {
		t.Fatalf("stash holds %q, want only x.txt", files)
	}
}
