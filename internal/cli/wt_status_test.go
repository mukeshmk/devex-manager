package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/mukeshmk/devex-manager/internal/testutil"
)

func TestStatusCommand(t *testing.T) {
	f := testutil.NewRepo(t)
	f.AddWorktree(t, "feat", "feat")
	t.Chdir(f.Root + "/main")
	in, _ := os.CreateTemp(t.TempDir(), "in")
	defer in.Close()
	for _, verb := range []string{"status", "list"} {
		var out, errw bytes.Buffer
		if rc := Run("git-wt", []string{verb}, in, &out, &errw); rc != 0 {
			t.Fatalf("%s rc=%d %s", verb, rc, errw.String())
		}
		if !strings.Contains(out.String(), "Total Worktrees: 2") || strings.Contains(out.String(), "\x1b") {
			t.Errorf("%s output:\n%s", verb, out.String())
		}
	}
}
