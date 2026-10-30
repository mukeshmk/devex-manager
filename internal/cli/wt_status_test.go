package cli

import (
	"bytes"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

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

// M6: non-TTY width follows $COLUMNS like dx skills list, instead of a fixed 100.
func TestStatusHonoursColumns(t *testing.T) {
	f := testutil.NewRepo(t)
	f.AddWorktree(t, "feat", "feature/a-very-long-branch-name-that-would-need-a-wide-terminal")
	t.Chdir(f.Root + "/main")
	in, _ := os.CreateTemp(t.TempDir(), "in")
	defer in.Close()
	for _, cols := range []int{60, 140} {
		t.Setenv("COLUMNS", strconv.Itoa(cols))
		var out bytes.Buffer
		Run("git-wt", []string{"status"}, in, &out, &out)
		widest := 0
		for _, l := range strings.Split(out.String(), "\n") {
			widest = max(widest, lipgloss.Width(l))
		}
		if widest > cols || (cols == 60 && widest < 50) {
			t.Errorf("COLUMNS=%d: widest line %d\n%s", cols, widest, out.String())
		}
	}
}
