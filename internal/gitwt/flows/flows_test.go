package flows

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mukeshmk/devex-manager/internal/gitwt/sync"
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
	err := Remove(e, "dirty", false)
	if !errors.Is(err, ErrReported) || !strings.Contains(b.String(), "Uncommitted changes") {
		t.Fatalf("err=%v out=%s", err, b)
	}
	if _, serr := os.Stat(d); serr != nil {
		t.Fatal("worktree removed")
	}
}
