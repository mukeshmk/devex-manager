package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mukeshmk/devex-manager/internal/testutil"
)

func runWt(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	in, _ := os.CreateTemp(t.TempDir(), "in")
	var o, e bytes.Buffer
	code := Run("git-wt", args, in, &o, &e)
	return code, o.String(), e.String()
}

func TestWtAddCLI(t *testing.T) {
	f := testutil.NewRepo(t)
	t.Chdir(f.Root)
	if code, _, e := runWt(t, "add"); code != 1 || !strings.Contains(e, "Error: Branch name is required") {
		t.Fatal(code, e)
	}
	code, o, e := runWt(t, "add", "DEVX-5-x")
	if code != 0 || !strings.Contains(o, "Worktree ready!") {
		t.Fatal(code, o, e)
	}
	if _, err := os.Stat(filepath.Join(f.Root, "DEVX-5")); err != nil {
		t.Fatal(err)
	}
}

func TestWtCloneCLI(t *testing.T) {
	f := testutil.NewRepo(t)
	t.Chdir(f.Tmp)
	if code, _, e := runWt(t, "clone"); code != 1 || !strings.Contains(e, "Git repository URL is required") {
		t.Fatal(code, e)
	}
	if code, o, e := runWt(t, "clone", f.Origin); code != 0 {
		t.Fatal(code, o, e)
	}
}
