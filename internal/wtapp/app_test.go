package wtapp

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/mukeshmk/devex-manager/internal/config"
	"github.com/mukeshmk/devex-manager/internal/ctx"
	"github.com/mukeshmk/devex-manager/internal/gitx"
	"github.com/mukeshmk/devex-manager/internal/testutil"
)

// start builds the app on a fixture with DEVX-1 and DEVX-2, loaded and sized, recording execs into got.
func start(t *testing.T, got *[][]string) (*testutil.Fixture, tea.Model) {
	t.Helper()
	f := testutil.NewRepo(t)
	f.AddWorktree(t, "DEVX-1", "feature/DEVX-1-long-branch-name")
	f.AddWorktree(t, "DEVX-2", "DEVX-2")
	repo, err := gitx.Open(f.Root)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadRepo(repo.Root)
	if err != nil {
		t.Fatal(err)
	}
	m := New(repo, cfg, "/path/to/dx").(model)
	m.exec = func(c *exec.Cmd) tea.Cmd {
		if c.Path != "/path/to/dx" {
			t.Errorf("exec path %q", c.Path)
		}
		*got = append(*got, c.Args)
		return nil
	}
	var tm tea.Model = m
	tm, _ = tm.Update(m.Init()())
	tm, _ = tm.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return f, tm
}

// press sends keys; names are special keys, anything else is typed as runes. Commands are run and fed back
// (one level), except exec commands, which the injected exec swallows.
func press(m tea.Model, keys ...string) tea.Model {
	special := map[string]tea.KeyType{"tab": tea.KeyTab, "space": tea.KeySpace, "esc": tea.KeyEsc,
		"enter": tea.KeyEnter, "up": tea.KeyUp, "down": tea.KeyDown, "backspace": tea.KeyBackspace}
	for _, k := range keys {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		if kt, ok := special[k]; ok {
			msg = tea.KeyMsg{Type: kt}
		}
		var cmd tea.Cmd
		m, cmd = m.Update(msg)
		if cmd != nil {
			if out := cmd(); out != nil {
				if _, quit := out.(tea.QuitMsg); !quit {
					m, _ = m.Update(out)
				}
			}
		}
	}
	return m
}

func checkWidth(t *testing.T, v string, w, h int) {
	t.Helper()
	lines := strings.Split(v, "\n")
	if len(lines) > h {
		t.Errorf("%d lines > height %d", len(lines), h)
	}
	for i, l := range lines {
		if lipgloss.Width(l) > w {
			t.Errorf("line %d width %d > %d: %q", i, lipgloss.Width(l), w, l)
		}
	}
}

func TestAppResize(t *testing.T) {
	var got [][]string
	_, m := start(t, &got)
	for _, sz := range [][2]int{{80, 24}, {200, 50}, {60, 12}} {
		m, _ = m.Update(tea.WindowSizeMsg{Width: sz[0], Height: sz[1]})
		v := m.View()
		checkWidth(t, v, sz[0], sz[1])
		if !strings.Contains(v, "DEVX-2") || !strings.Contains(v, "q quit") {
			t.Errorf("%v view missing content:\n%s", sz, v)
		}
	}
}

func TestAppTooSmall(t *testing.T) {
	var got [][]string
	_, m := start(t, &got)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 50, Height: 10})
	if v := m.View(); v != "Terminal too small (need 60×12)" {
		t.Errorf("view %q", v)
	}
}

func TestAppLoading(t *testing.T) {
	repo := &gitx.Repo{}
	m, _ := New(repo, config.RepoConfig{}, "x").Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if !strings.Contains(m.View(), "Loading…") {
		t.Errorf("no placeholder:\n%s", m.View())
	}
}

func TestAppToggleTodo(t *testing.T) {
	var got [][]string
	f, m := start(t, &got)
	p := ctx.Path(filepath.Join(f.Root, ".git"), "DEVX-2")
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, []byte("# Context: DEVX-2\n\n## Todos\n- [ ] a\n\n## Notes\n"), 0o644)
	m = press(m, "/", "DEVX-2", "enter", "tab", "space")
	b, _ := os.ReadFile(p)
	if !strings.Contains(string(b), "- [x] a") {
		t.Errorf("file not toggled:\n%s", b)
	}
	if !strings.Contains(m.View(), "[✓] a") {
		t.Errorf("view not refreshed:\n%s", m.View())
	}
	// No ctx file: "No todos" and toggling is a no-op.
	m = press(m, "tab", "/", "backspace", "1", "enter", "tab", "space")
	if !strings.Contains(m.View(), "No todos") {
		t.Errorf("want No todos:\n%s", m.View())
	}
}

func TestAppFilter(t *testing.T) {
	var got [][]string
	_, m := start(t, &got)
	m = press(m, "/", "DEVX-2")
	v := m.View()
	if !strings.Contains(v, "DEVX-2") || strings.Contains(v, "DEVX-1") || strings.Contains(v, " main ") {
		t.Errorf("filter not applied:\n%s", v)
	}
	m = press(m, "esc")
	if !strings.Contains(m.View(), "DEVX-1") {
		t.Errorf("esc did not clear filter:\n%s", m.View())
	}
}

func TestAppDeleteShowsConfirm(t *testing.T) {
	var got [][]string
	f, m := start(t, &got)
	os.WriteFile(filepath.Join(f.Root, "DEVX-2", "dirty.txt"), []byte("x"), 0o644)
	m = press(m, "r", "/", "DEVX-2", "enter", "d")
	if v := m.View(); !strings.Contains(v, "dirty.txt") {
		t.Errorf("confirm missing file:\n%s", v)
	}
	checkWidth(t, m.View(), 100, 30)
	m = press(m, "n")
	if len(got) != 0 || strings.Contains(m.View(), "n cancel") {
		t.Errorf("cancel: execs %v\n%s", got, m.View())
	}
	m = press(m, "d", "esc")
	if len(got) != 0 {
		t.Errorf("esc cancel ran %v", got)
	}
	press(m, "d", "y")
	if len(got) != 1 || !slices.Equal(got[0], []string{"git-wt", "rm", "DEVX-2"}) {
		t.Errorf("execs %v", got)
	}
}

func TestAppDeleteCleanSaysNothingLost(t *testing.T) {
	var got [][]string
	_, m := start(t, &got)
	m = press(m, "/", "DEVX-2", "enter", "d")
	if !strings.Contains(m.View(), "nothing would be lost") {
		t.Errorf("view:\n%s", m.View())
	}
}

func TestAppSyncCleanExec(t *testing.T) {
	var got [][]string
	_, m := start(t, &got)
	m = press(m, "s", "c", "enter")
	if len(got) != 2 || !slices.Equal(got[0], []string{"git-wt", "sync"}) || !slices.Equal(got[1], []string{"git-wt", "clean"}) {
		t.Errorf("execs %v", got)
	}
}
