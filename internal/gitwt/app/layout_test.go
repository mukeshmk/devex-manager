package wtapp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

var titles = []string{"[1] Status", "[2] Worktrees", "[3] Context", "[4] Sync", "Command log"}

func resize(m tea.Model, w, h int) tea.Model {
	m, _ = m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return m
}

func hasAll(t *testing.T, v string, want ...string) {
	t.Helper()
	for _, s := range want {
		if !strings.Contains(v, s) {
			t.Errorf("missing %q:\n%s", s, v)
		}
	}
}

func TestLayoutWide(t *testing.T) {
	_, m := start(t)
	v := resize(m, 120, 40).View()
	hasAll(t, v, titles...)
	hasAll(t, v, "DEVX-2", "? keys", "sync: no copy paths")
	checkWidth(t, v, 120, 40)
}

func TestLayoutStacked(t *testing.T) {
	_, m := start(t)
	v := resize(m, 70, 30).View()
	hasAll(t, v, titles...)
	checkWidth(t, v, 70, 30)
	if _, _, _, stacked := layout(70, 30); !stacked {
		t.Error("70 columns should stack")
	}
}

func TestLayoutTooSmall(t *testing.T) {
	_, m := start(t)
	if v := resize(m, 50, 10).View(); v != "Terminal too small (need 60×12)" {
		t.Errorf("view %q", v)
	}
}

func TestPanelKeys(t *testing.T) {
	_, m := start(t)
	m = press(m, "3")
	if !strings.Contains(m.View(), "[3] Context"+focusMark) || strings.Contains(m.View(), "[2] Worktrees"+focusMark) {
		t.Errorf("Context not marked focused:\n%s", m.View())
	}
	for _, step := range []struct {
		key   string
		focus panel
		main  bool
	}{{"tab", panelSync, false}, {"shift+tab", panelContext, false}, {"enter", panelContext, true}, {"esc", panelContext, false},
		{"5", panelLog, false}, {"tab", panelStatus, false}, {"shift+tab", panelLog, false}} {
		m = press(m, step.key)
		if mm := m.(model); mm.focus != step.focus || mm.focusMain != step.main {
			t.Errorf("after %s: focus %d main %v", step.key, mm.focus, mm.focusMain)
		}
		if step.main && !strings.Contains(m.View(), "Context"+focusMark+" ") {
			t.Errorf("main view not marked focused:\n%s", m.View())
		}
	}
	// The focused log scrolls back from its tail.
	mm := m.(model)
	for i := range 40 {
		mm.log.Add(fmt.Sprintf("line-%02d", i))
	}
	m = press(mm, "up", "up")
	if v := m.View(); !strings.Contains(v, "line-37") || strings.Contains(v, "line-39") {
		t.Errorf("log not scrolled:\n%s", v)
	}
	// Leaving the log returns it to its tail.
	if v := press(m, "tab", "shift+tab").View(); !strings.Contains(v, "line-39") {
		t.Errorf("log scroll kept after leaving:\n%s", v)
	}
}

func TestResizeWithPopupOpen(t *testing.T) {
	_, m := start(t)
	m = press(m, "?")
	hasAll(t, m.View(), "Keys", "C ", "clear done todos")
	m = resize(m, 60, 12)
	checkWidth(t, m.View(), 60, 12)
	if !strings.Contains(m.View(), "Keys") {
		t.Errorf("popup gone after resize:\n%s", m.View())
	}
	m = press(m, "x") // any key closes help
	if m.(model).popup != nil {
		t.Error("help still open")
	}
}

// Width safety at every size, focus and popup state, with long content.
func TestWidthEverySize(t *testing.T) {
	f, m := start(t)
	f.AddWorktree(t, "a-really-long-worktree-folder-name-that-will-not-fit", "feature/another-extremely-long-branch-name-here")
	p := filepath.Join(f.Root, ".git", "info", "devex", "contexts", "DEVX-2.md")
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, []byte("# Context\n\n## Todos\n- [ ] "+strings.Repeat("wide 漢字 ", 30)+"\n\n## Notes\n\tindented\twith tabs "+strings.Repeat("x", 300)+"\n"), 0o644)
	mm := m.(model)
	mm.log.Add(strings.Repeat("log ", 100))
	m = press(mm, "r", "/", "DEVX-2", "enter")
	for _, keys := range [][]string{nil, {"1"}, {"3"}, {"3", "enter"}, {"4", "enter"}, {"5"}, {"?"}} {
		mk := press(m, keys...)
		for w := 60; w <= 130; w += 7 {
			for h := 12; h <= 40; h += 5 {
				checkWidth(t, resize(mk, w, h).View(), w, h)
			}
		}
	}
}

func TestSyncPanels(t *testing.T) {
	f, m := start(t)
	os.WriteFile(filepath.Join(f.Root, "DEVX-1", ".env"), []byte("A=1"), 0o644)
	mm := m.(model)
	mm.cfg.CopyPaths = []string{".env"}
	m = press(mm, "r")
	hasAll(t, m.View(), "sync: 1 files out of sync", "Out of sync: .env")
	hasAll(t, press(m, "4").View(), "[4] Sync"+focusMark, ".env")
}

// Leaving the main view drops its scroll, so another worktree's details start at the top.
func TestMainScrollResets(t *testing.T) {
	f, m := start(t)
	for _, d := range []string{"DEVX-1", "DEVX-2"} {
		for i := range 40 {
			os.WriteFile(filepath.Join(f.Root, d, fmt.Sprintf("f%02d.txt", i)), nil, 0o644)
		}
	}
	m = press(m, "r", "enter") // DEVX-1 is the first row
	for range 30 {
		m = press(m, "down")
	}
	if strings.Contains(m.View(), "Branch") {
		t.Fatalf("main view did not scroll:\n%s", m.View())
	}
	m = press(m, "esc", "down")
	if v := m.View(); !strings.Contains(v, "Branch") {
		t.Errorf("stale scroll hides Branch:\n%s", v)
	}
}
