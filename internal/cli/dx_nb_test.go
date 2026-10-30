package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func nbRun(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	in, _ := os.CreateTemp(t.TempDir(), "in")
	var o, e bytes.Buffer
	c := Run("dx", append([]string{"nb"}, args...), in, &o, &e)
	return c, o.String(), e.String()
}

// fakeBin installs a script named name on PATH (only that dir, plus /usr/bin:/bin for sh basics).
func fakeBin(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestNbHelp(t *testing.T) {
	for _, a := range [][]string{nil, {"bogus"}} {
		c, o, _ := nbRun(t, a...)
		if c != 1 || !strings.Contains(o, "Usage: dx nb <command> [arguments]") || !strings.Contains(o, "list    - Recursive") {
			t.Fatalf("%v %d %q", a, c, o)
		}
	}
}

func TestNbStrip(t *testing.T) {
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	c, o, _ := nbRun(t, "strip")
	if c != 1 || !strings.Contains(o, "Usage: dx nb strip") {
		t.Fatal(c, o)
	}
	c, _, e := nbRun(t, "strip", "a.ipynb")
	if c != 1 || !strings.Contains(e, "Neither 'nbstripout' nor 'jupyter'") {
		t.Fatal(c, e)
	}
	fakeBin(t, bin, "jupyter", `echo "jupyter $@"`)
	c, o, _ = nbRun(t, "strip", "a.ipynb", "b.ipynb")
	if c != 0 || !strings.Contains(o, "jupyter nbconvert") || !strings.Contains(o, "jupyter nbconvert --clear-output --inplace a.ipynb b.ipynb") || !strings.Contains(o, "Done.") {
		t.Fatal(c, o)
	}
	fakeBin(t, bin, "nbstripout", `echo "nbstripout $@"; exit 3`)
	c, o, _ = nbRun(t, "strip", "a.ipynb")
	if c != 3 || !strings.Contains(o, "nbstripout a.ipynb") || strings.Contains(o, "Done.") {
		t.Fatal(c, o)
	}
}

func TestNbDiff(t *testing.T) {
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	if c, o, _ := nbRun(t, "diff"); c != 1 || !strings.Contains(o, "Usage: dx nb diff") {
		t.Fatal(c, o)
	}
	fakeBin(t, bin, "git", `echo "git $@"`)
	c, o, e := nbRun(t, "diff", "a.ipynb", "--stat")
	if c != 0 || !strings.Contains(o, "git diff a.ipynb --stat") || !strings.Contains(e, "'nbdime' is not installed") {
		t.Fatal(c, o, e)
	}
	fakeBin(t, bin, "nbdiff", `echo "nbdiff $@"; exit 2`)
	c, o, e = nbRun(t, "diff", "a.ipynb", "b.ipynb")
	if c != 2 || !strings.Contains(o, "nbdiff a.ipynb b.ipynb") || e != "" {
		t.Fatal(c, o, e)
	}
}

func TestNbKernel(t *testing.T) {
	t.Chdir(t.TempDir())
	if c, _, e := nbRun(t, "kernel"); c != 1 || !strings.Contains(e, "No .venv found") {
		t.Fatal(c, e)
	}
	os.MkdirAll(".venv/bin", 0o755)
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	fakeBin(t, bin, "python", `echo "py $@"; [ "$3" = "--version" ] && exit 1; exit 0`)
	if c, _, e := nbRun(t, "kernel"); c != 1 || !strings.Contains(e, "'ipykernel' is not installed") {
		t.Fatal(c, e)
	}
	fakeBin(t, bin, "python", `echo "py $@"`)
	wd, _ := os.Getwd()
	base := filepath.Base(wd)
	c, o, _ := nbRun(t, "kernel")
	if c != 0 || !strings.Contains(o, "py -m ipykernel install --user --name "+base+" --display-name DevEx: "+base) || !strings.Contains(o, "registered successfully") {
		t.Fatal(c, o)
	}
}

func TestNbList(t *testing.T) {
	t.Chdir(t.TempDir())
	if c, o, _ := nbRun(t, "list"); c != 0 || !strings.Contains(o, "No notebooks found") {
		t.Fatal(c, o)
	}
	os.WriteFile("dirty.ipynb", []byte(`{"cells":[{"cell_type":"code","outputs":[{"output_type":"x"}]},{"cell_type":"markdown"}]}`), 0o644)
	os.WriteFile("clean.ipynb", []byte(`{"cells":[{"cell_type":"code","outputs":[]}]}`), 0o644)
	os.WriteFile("bad.ipynb", []byte(`{`), 0o644)
	c, o, _ := nbRun(t, "list")
	for _, w := range []string{"● Dirty", "○ Stripped", "Error (JSON)", "2 (1/1)", "Total Notebooks: 3", "Total Cells:     3 (2 Code, 1 Markdown)", "1 Stripped, 1 Dirty"} {
		if c != 0 || !strings.Contains(o, w) {
			t.Fatalf("missing %q in %d %s", w, c, o)
		}
	}
}
