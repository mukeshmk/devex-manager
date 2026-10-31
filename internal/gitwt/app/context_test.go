package wtapp

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mukeshmk/devex-manager/internal/ctx"
	"github.com/mukeshmk/devex-manager/internal/shared/testutil"
)

// ctxFile writes DEVX-2's context file with the given todo lines and returns its path.
func ctxFile(t *testing.T, f *testutil.Fixture, todos string) string {
	t.Helper()
	p := ctx.Path(filepath.Join(f.Root, ".git"), "DEVX-2")
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte("# Context: DEVX-2\n\n## Todos\n"+todos+"\n## Notes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestCtxAddTodo(t *testing.T) {
	f, m := start(t)
	m = press(m, "/", "DEVX-2", "enter", "3", "a")
	if v := m.View(); !strings.Contains(v, "New todo for DEVX-2") {
		t.Fatalf("no input popup:\n%s", v)
	}
	m = press(m, "ship it", "enter")
	p := ctx.Path(filepath.Join(f.Root, ".git"), "DEVX-2")
	if !strings.Contains(read(t, p), "- [ ] ship it") {
		t.Errorf("todo not added:\n%s", read(t, p))
	}
	hasAll(t, m.View(), "[ ] ship it", "ctx: add ship it")
}

func TestCtxToggleAndDelete(t *testing.T) {
	f, m := start(t)
	p := ctxFile(t, f, "- [ ] one\n- [ ] two\n")
	m = press(m, "/", "DEVX-2", "enter", "3", "down", "space")
	if !strings.Contains(read(t, p), "- [x] two") {
		t.Errorf("not toggled:\n%s", read(t, p))
	}
	hasAll(t, m.View(), "ctx: toggle two")
	m = press(m, "x")
	hasAll(t, m.View(), `Delete todo "two"?`)
	m = press(m, "n")
	if !strings.Contains(read(t, p), "two") {
		t.Fatal("n must not delete")
	}
	m = press(m, "x", "y")
	if strings.Contains(read(t, p), "two") {
		t.Errorf("not deleted:\n%s", read(t, p))
	}
	hasAll(t, m.View(), "ctx: delete two", "[ ] one") // cursor clamped onto the last todo
	m = press(m, "space")
	if !strings.Contains(read(t, p), "- [x] one") {
		t.Errorf("cursor not clamped:\n%s", read(t, p))
	}
}

func TestCtxClearDone(t *testing.T) {
	f, m := start(t)
	p := ctxFile(t, f, "- [x] a\n- [ ] b\n- [x] c\n")
	m = press(m, "/", "DEVX-2", "enter", "3", "C")
	hasAll(t, m.View(), "Clear 2 done todo(s)?")
	m = press(m, "y")
	got := read(t, p)
	if strings.Contains(got, "[x]") || !strings.Contains(got, "- [ ] b") {
		t.Errorf("not cleared:\n%s", got)
	}
	hasAll(t, m.View(), "ctx: clear 2 done")
}

func TestCtxDetachedNoop(t *testing.T) {
	f, m := start(t)
	cmd := exec.Command("git", "-C", filepath.Join(f.Root, "DEVX-2"), "checkout", "-q", "--detach")
	cmd.Env = f.Env()
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	m, _ = m.Update(m.(model).load()())
	m = press(m, "/", "DEVX-2", "enter", "3", "a")
	hasAll(t, m.View(), "No todos (detached HEAD)")
	if m.(model).popup != nil {
		t.Error("a must do nothing when detached")
	}
}
