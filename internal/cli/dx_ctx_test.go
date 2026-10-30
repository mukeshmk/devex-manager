package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mukeshmk/devex-manager/internal/shared/testutil"
)

func ctxRun(t *testing.T, cwd, stdin string, args ...string) (int, string, string) {
	t.Helper()
	t.Chdir(cwd)
	in, err := os.CreateTemp(t.TempDir(), "in")
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	in.WriteString(stdin)
	in.Seek(0, 0)
	var out, errb bytes.Buffer
	rc := Run("dx", append([]string{"ctx"}, args...), in, &out, &errb)
	return rc, out.String(), errb.String()
}

func TestCtxFlow(t *testing.T) { // tests/test_dx_ctx.sh
	f := testutil.NewRepo(t)
	wt := f.AddWorktree(t, "fa", "feature/a")
	rc, out, _ := ctxRun(t, wt, "")
	if rc != 0 || !strings.Contains(out, "Context: feature/a") || !strings.Contains(out, "No tasks. Add one with 'dx ctx todo add <task>'") {
		t.Fatalf("rc=%d %s", rc, out)
	}
	if _, err := os.Stat(filepath.Join(f.Root, ".git", "info", "devex", "contexts", "feature__a.md")); err != nil {
		t.Fatal(err)
	}
	ctxRun(t, wt, "", "todo", "add", "First task")
	ctxRun(t, wt, "", "add", "Second", "task")
	ctxRun(t, wt, "", "Third", "bare") // unknown word becomes add
	_, out, _ = ctxRun(t, wt, "", "show")
	for _, w := range []string{"[1] [ ] First task", "[2] [ ] Second task", "[3] [ ] Third bare", "Todos:", "Notes:"} {
		if !strings.Contains(out, w) {
			t.Errorf("missing %q in %s", w, out)
		}
	}
	ctxRun(t, wt, "", "done", "1")
	ctxRun(t, wt, "", "todo", "check", "2")
	_, out, _ = ctxRun(t, wt, "", "todo", "list")
	if !strings.Contains(out, "[1] [✓] First task") || !strings.Contains(out, "[2] [✓] Second task") {
		t.Fatalf("%s", out)
	}
	ctxRun(t, wt, "", "undo", "1")
	ctxRun(t, wt, "", "todo", "uncheck", "2")
	_, out, _ = ctxRun(t, wt, "", "")
	if !strings.Contains(out, "[1] [ ] First task") || !strings.Contains(out, "[2] [ ] Second task") {
		t.Fatalf("%s", out)
	}
	ctxRun(t, wt, "", "rm", "1")
	ctxRun(t, wt, "", "todo", "delete", "1")
	ctxRun(t, wt, "", "done", "1")
	rc, out, _ = ctxRun(t, wt, "", "todo", "clear")
	if rc != 0 || !strings.Contains(out, "✓ Cleared 1 completed tasks.") {
		t.Fatalf("%d %s", rc, out)
	}
	_, out, _ = ctxRun(t, wt, "", "show")
	if strings.Contains(out, "Third") || !strings.Contains(out, "No tasks") {
		t.Fatalf("%s", out)
	}
}

func TestCtxErrors(t *testing.T) {
	f := testutil.NewRepo(t)
	main := filepath.Join(f.Root, "main")
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"done"}, "Error: Specify a todo index to check."},
		{[]string{"undo"}, "Error: Specify a todo index to uncheck."},
		{[]string{"rm"}, "Error: Specify a todo index to delete."},
		{[]string{"done", "5"}, "Error: Todo index 5 not found."},
		{[]string{"rm", "abc"}, "Error: Todo index abc not found."},
		{[]string{"add"}, "Error: Task description cannot be empty."},
		{[]string{"todo", "bogus"}, "Unknown todo sub-command: bogus"},
	} {
		rc, out, errs := ctxRun(t, main, "", c.args...)
		if rc != 1 || !strings.Contains(out+errs, c.want) {
			t.Errorf("%v: rc=%d out=%q err=%q", c.args, rc, out, errs)
		}
	}
}

func TestCtxDetachedAndNotRepo(t *testing.T) {
	f := testutil.NewRepo(t)
	main := filepath.Join(f.Root, "main")
	git(t, main, "checkout", "-q", "--detach")
	rc, _, errs := ctxRun(t, main, "")
	if rc != 1 || !strings.Contains(errs, "Error: Not currently on a branch (detached HEAD)") {
		t.Fatalf("%d %q", rc, errs)
	}
	rc, _, errs = ctxRun(t, t.TempDir(), "")
	if rc != 1 || !strings.Contains(errs, "Error: Not a git repository") {
		t.Fatalf("%d %q", rc, errs)
	}
}

func TestCtxHelp(t *testing.T) {
	f := testutil.NewRepo(t)
	for _, a := range []string{"help", "-h", "--help"} {
		rc, out, _ := ctxRun(t, filepath.Join(f.Root, "main"), "", a)
		if rc != 0 || !strings.Contains(out, "Usage: dx ctx [command] [arguments]") || !strings.Contains(out, "todo clear") {
			t.Errorf("%s: %d %s", a, rc, out)
		}
	}
}

func TestCtxClean(t *testing.T) {
	f := testutil.NewRepo(t)
	main := filepath.Join(f.Root, "main")
	ctxRun(t, main, "") // creates main.md
	dir := filepath.Join(f.Root, ".git", "info", "devex", "contexts")
	os.WriteFile(filepath.Join(dir, "old-feature.md"), nil, 0o644)

	rc, out, _ := ctxRun(t, main, "", "clean") // EOF means no
	if rc != 0 || !strings.Contains(out, "Found 1 orphan contexts") || !strings.Contains(out, "Cleanup cancelled.") {
		t.Fatalf("%d %s", rc, out)
	}
	if _, err := os.Stat(filepath.Join(dir, "old-feature.md")); err != nil {
		t.Fatal("deleted without consent")
	}
	rc, out, _ = ctxRun(t, main, "y\n", "clean")
	if rc != 0 || !strings.Contains(out, "✓ Deleted context file for old-feature") || !strings.Contains(out, "Cleanup complete!") {
		t.Fatalf("%d %s", rc, out)
	}
	if _, err := os.Stat(filepath.Join(dir, "old-feature.md")); err == nil {
		t.Fatal("orphan kept")
	}
	if _, err := os.Stat(filepath.Join(dir, "main.md")); err != nil {
		t.Fatal("active context deleted")
	}
	_, out, _ = ctxRun(t, main, "", "clean")
	if !strings.Contains(out, "No orphan contexts found. System is clean!") {
		t.Fatal(out)
	}
}

func TestCtxEdit(t *testing.T) {
	f := testutil.NewRepo(t)
	main := filepath.Join(f.Root, "main")
	ed := filepath.Join(t.TempDir(), "ed.sh")
	os.WriteFile(ed, []byte("#!/bin/sh\necho edited >> \"$1\"\n"), 0o755)
	t.Setenv("EDITOR", ed)
	if rc, out, errs := ctxRun(t, main, "", "edit"); rc != 0 {
		t.Fatalf("%d %s %s", rc, out, errs)
	}
	b, _ := os.ReadFile(filepath.Join(f.Root, ".git", "info", "devex", "contexts", "main.md"))
	if !strings.HasSuffix(string(b), "edited\n") {
		t.Fatalf("%q", b)
	}
}
