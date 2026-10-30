package nb

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func write(t *testing.T, p, s string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestInspect(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.ipynb")
	write(t, p, `{"cells":[{"cell_type":"code","outputs":[{"output_type":"stream"}]},{"cell_type":"code","outputs":[]},{"cell_type":"markdown"}]}`)
	i := Inspect(p)
	if i.Err != nil || i.Cells != 3 || i.Code != 2 || i.Markdown != 1 || !i.HasOutput || i.Size == 0 {
		t.Fatalf("%+v", i)
	}
}

func TestInspectBadJSON(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.ipynb")
	write(t, p, "{nope")
	if Inspect(p).Err == nil {
		t.Fatal("want error")
	}
}

func TestFindSkipsHidden(t *testing.T) {
	d := t.TempDir()
	for _, f := range []string{"a.ipynb", "sub/b.ipynb", ".venv/c.ipynb", "sub/.hid/d.ipynb", "e.txt"} {
		write(t, filepath.Join(d, f), "{}")
	}
	got := Find(d)
	sort.Strings(got)
	if len(got) != 2 || got[0] != "a.ipynb" || got[1] != filepath.Join("sub", "b.ipynb") {
		t.Fatalf("%v", got)
	}
}
