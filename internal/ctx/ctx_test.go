package ctx

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const base = "# Context: b\n\n## Todos\n\n## Notes\nhello\n"

func texts(c string) []string {
	var r []string
	for _, t := range ParseTodos(c) {
		r = append(r, t.Text)
	}
	return r
}

func TestCtxPathSlashes(t *testing.T) {
	got := Path("/c", "feature/a")
	if want := filepath.Join("/c", "info", "devex", "contexts", "feature__a.md"); got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestEnsureFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x", "b.md")
	if err := EnsureFile(p, "feature/a"); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	if string(b) != "# Context: feature/a\n\n## Todos\n\n## Notes\n" {
		t.Fatalf("template: %q", b)
	}
	os.WriteFile(p, []byte("keep"), 0o644)
	EnsureFile(p, "feature/a")
	if b, _ := os.ReadFile(p); string(b) != "keep" {
		t.Fatal("overwrote existing file")
	}
}

func TestAddAndParse(t *testing.T) {
	c := AddTodo(base, "First task")
	c = AddTodo(c, "Second task")
	if got := texts(c); !reflect.DeepEqual(got, []string{"First task", "Second task"}) {
		t.Fatalf("got %v\n%s", got, c)
	}
	if want := "# Context: b\n\n## Todos\n- [ ] First task\n- [ ] Second task\n\n## Notes\nhello\n"; c != want {
		t.Fatalf("got %q", c)
	}
	if td := ParseTodos(c); td[1].Index != 2 || td[1].Line != 5 || td[1].Done {
		t.Fatalf("%+v", td[1])
	}
}

func TestAddCreatesSection(t *testing.T) {
	c := AddTodo("# Context: b\n", "x")
	if c != "# Context: b\n\n## Todos\n- [ ] x\n" {
		t.Fatalf("%q", c)
	}
}

func TestCaseInsensitiveHeadingAndOutsideIgnored(t *testing.T) {
	c := "- [ ] before\n## TODOS\n- [x] in\n  - [ ] nested\n## Notes\n- [ ] notes item\n# Top\n- [ ] top\n"
	if got := texts(c); !reflect.DeepEqual(got, []string{"in", "nested"}) {
		t.Fatalf("got %v", got)
	}
	if !ParseTodos(c)[0].Done {
		t.Fatal("want done")
	}
	if got := Notes(c); !reflect.DeepEqual(got, []string{"- [ ] notes item"}) {
		t.Fatalf("notes %v", got)
	}
}

func TestSetDoneUndoRm(t *testing.T) {
	c := AddTodo(AddTodo(base, "First task"), "Second task")
	c, err := SetDone(c, 1, true)
	if err != nil || !ParseTodos(c)[0].Done {
		t.Fatalf("%v\n%s", err, c)
	}
	c, _ = SetDone(c, 1, false)
	if ParseTodos(c)[0].Done {
		t.Fatal("undo failed")
	}
	c, err = RemoveTodo(c, 1)
	if err != nil || !reflect.DeepEqual(texts(c), []string{"Second task"}) || ParseTodos(c)[0].Index != 1 {
		t.Fatalf("%v %s", err, c)
	}
	for _, i := range []int{0, 2, -1} {
		if _, err := SetDone(c, i, true); err == nil {
			t.Errorf("SetDone %d: want error", i)
		}
		if _, err := RemoveTodo(c, i); err == nil {
			t.Errorf("RemoveTodo %d: want error", i)
		}
	}
}

func TestClearDone(t *testing.T) {
	c := AddTodo(AddTodo(base, "a"), "b")
	c, _ = SetDone(c, 1, true)
	c = strings.Replace(c, "hello", "- [x] note stays", 1)
	c, n := ClearDone(c)
	if n != 1 || !reflect.DeepEqual(texts(c), []string{"b"}) || !strings.Contains(c, "- [x] note stays") {
		t.Fatalf("n=%d\n%s", n, c)
	}
	if HasTodos("# x\n") || !HasTodos(c) {
		t.Fatal("HasTodos")
	}
}

func TestOrphans(t *testing.T) {
	cd := t.TempDir()
	for _, b := range []string{"main", "old-feature", "feature/gone"} {
		p := Path(cd, b)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, nil, 0o644)
	}
	got := Orphans(cd, func(b string) bool { return b == "main" })
	if !reflect.DeepEqual(got, []string{"feature/gone", "old-feature"}) {
		t.Fatalf("got %v", got)
	}
	if got := Orphans(t.TempDir(), func(string) bool { return false }); len(got) != 0 {
		t.Fatalf("no dir: %v", got)
	}
}
