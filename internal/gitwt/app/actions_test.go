package wtapp

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mukeshmk/devex-manager/internal/shared/config"
	"github.com/mukeshmk/devex-manager/internal/shared/gitx"
	"github.com/mukeshmk/devex-manager/internal/shared/testutil"
)

// drain feeds up to n events from the flow channel through Update, stopping after a prompt or the flow's end.
// Returned commands are dropped: they only re-arm waitEvent or reload.
func drain(t *testing.T, m tea.Model, n int) tea.Model {
	t.Helper()
	ch := m.(model).events
	for range n {
		select {
		case msg := <-ch:
			m, _ = m.Update(msg)
			switch msg.(type) {
			case promptMsg, flowDoneMsg:
				return m
			}
		case <-time.After(10 * time.Second):
			t.Fatal("timed out waiting for a flow event")
		}
	}
	return m
}

func write(t *testing.T, p, body string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

func logHas(m tea.Model, s string) bool {
	return slices.ContainsFunc(m.(model).log.Lines(), func(l string) bool { return strings.Contains(l, s) })
}

// syncApp is start with .vscode as a copy path and a feat worktree.
func syncApp(t *testing.T) (main, feat string, m tea.Model) {
	t.Helper()
	f := testutil.NewRepo(t)
	main = filepath.Join(f.Root, "main")
	feat = f.AddWorktree(t, "feat", "feat")
	write(t, filepath.Join(f.Root, ".devex.conf"), "[copies]\npaths = .vscode\n")
	repo, err := gitx.Open(f.Root)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadRepo(repo.Root)
	if err != nil {
		t.Fatal(err)
	}
	mm := New(repo, cfg, main).(model)
	m, _ = mm.Update(mm.load()())
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return main, feat, m
}

func TestAppSyncInApp(t *testing.T) {
	main, feat, m := syncApp(t)
	write(t, filepath.Join(feat, ".vscode", "a.json"), "feat")
	m = drain(t, press(m, "s"), 100)
	p := m.(model).popup
	if p == nil || p.kind != popupConfirm || !strings.Contains(p.title, "Apply these changes?") {
		t.Fatalf("want confirm popup, got %+v", p)
	}
	m = drain(t, press(m, "enter"), 100)
	if got, _ := os.ReadFile(filepath.Join(main, ".vscode", "a.json")); string(got) != "feat" {
		t.Errorf("main has %q", got)
	}
	if s := m.(model).statusMsg; !strings.Contains(s, "sync: done") || m.(model).busy != "" {
		t.Errorf("status %q busy %q", s, m.(model).busy)
	}
	if !logHas(m, "Synchronization complete") {
		t.Errorf("log %v", m.(model).log.Lines())
	}
}

func TestAppSyncAbortInPopup(t *testing.T) {
	main, feat, m := syncApp(t)
	write(t, filepath.Join(main, ".vscode", "a.json"), "main")
	write(t, filepath.Join(feat, ".vscode", "a.json"), "feat")
	m = drain(t, press(m, "s"), 100)
	m = drain(t, press(m, "enter"), 100)
	if p := m.(model).popup; p == nil || p.kind != popupChoose {
		t.Fatalf("want conflict choose popup, got %+v", p)
	}
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd != nil {
		if _, quit := cmd().(tea.QuitMsg); quit {
			t.Fatal("ctrl+c in a flow popup quit the app")
		}
	}
	m = drain(t, m, 100)
	if s := m.(model).statusMsg; !strings.Contains(s, "sync aborted") {
		t.Errorf("status %q", s)
	}
	for p, want := range map[string]string{filepath.Join(main, ".vscode", "a.json"): "main", filepath.Join(feat, ".vscode", "a.json"): "feat"} {
		if got, _ := os.ReadFile(p); string(got) != want {
			t.Errorf("%s changed to %q", p, got)
		}
	}
}

func TestAppQuitBlockedWhileBusy(t *testing.T) {
	_, feat, m := syncApp(t)
	write(t, filepath.Join(feat, ".vscode", "a.json"), "feat")
	m = drain(t, press(m, "s"), 100)
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd != nil {
		if _, quit := cmd().(tea.QuitMsg); quit {
			t.Fatal("q quit while busy")
		}
	}
	if s := m.(model).statusMsg; !strings.Contains(s, "wait for sync to finish") {
		t.Errorf("status %q", s)
	}
	if !strings.Contains(m.View(), "wait for sync to finish") {
		t.Errorf("footer hides the refusal:\n%s", m.View())
	}
	m = drain(t, press(m, "esc"), 100) // answer no
	if m.(model).busy != "" {
		t.Errorf("still busy %q", m.(model).busy)
	}
	// Outside a popup, q is also refused while busy; s, c and d start nothing.
	mm := m.(model)
	mm.busy = "sync"
	m = press(mm, "q", "s", "c", "d")
	if mm := m.(model); mm.busy != "sync" || mm.popup != nil || mm.confirm != nil {
		t.Errorf("busy keys acted: busy %q popup %+v confirm %+v", mm.busy, mm.popup, mm.confirm)
	}
	m = press(mm, "q")
	if !strings.Contains(m.(model).statusMsg, "wait for sync to finish") {
		t.Errorf("status %q", m.(model).statusMsg)
	}
}

func TestAppCleanChecklist(t *testing.T) {
	f, m := start(t)
	dirty, clean := filepath.Join(f.Root, "DEVX-1"), filepath.Join(f.Root, "DEVX-2")
	write(t, filepath.Join(dirty, "x.txt"), "unsaved")
	m = drain(t, press(m, "c"), 100)
	p := m.(model).popup
	if p == nil || p.kind != popupChecklist || len(p.options) != 2 {
		t.Fatalf("want 2-row checklist, got %+v", p)
	}
	for i, o := range p.options {
		if p.ticks[i] != strings.HasPrefix(o, "DEVX-2:") {
			t.Errorf("row %q ticked=%v", o, p.ticks[i])
		}
	}
	m = drain(t, press(m, "enter"), 100)
	if exists(clean) || !exists(dirty) {
		t.Errorf("clean exists=%v dirty exists=%v; status %q", exists(clean), exists(dirty), m.(model).statusMsg)
	}
}

func TestAppDeleteInApp(t *testing.T) {
	f, m := start(t)
	d1, d2 := filepath.Join(f.Root, "DEVX-1"), filepath.Join(f.Root, "DEVX-2")
	write(t, filepath.Join(d1, "x.txt"), "unsaved")
	m = press(m, "r", "/", "DEVX-1", "enter", "d", "y") // dirty: y does nothing
	if m.(model).busy != "" || !exists(d1) {
		t.Fatalf("dirty delete started %q", m.(model).busy)
	}
	m = press(m, "esc", "esc", "/", "DEVX-2", "enter", "d", "y")
	if m.(model).busy == "" {
		t.Fatal("delete did not start")
	}
	m = drain(t, m, 100)
	if _, err := gitx.Run(f.Root, "show-ref", "--verify", "--quiet", "refs/heads/DEVX-2"); exists(d2) || err == nil {
		t.Errorf("DEVX-2 folder exists=%v branch err=%v; status %q", exists(d2), err, m.(model).statusMsg)
	}
	if !exists(d1) {
		t.Error("DEVX-1 removed")
	}
}

func TestAppNoCopyPaths(t *testing.T) {
	_, m := start(t)
	m = drain(t, press(m, "s"), 100)
	mm := m.(model)
	if mm.popup != nil || !strings.Contains(mm.statusMsg, "No copy paths configured") || !logHas(m, "No copy paths configured") {
		t.Errorf("popup %+v status %q log %v", mm.popup, mm.statusMsg, mm.log.Lines())
	}
}

// A delete assessment that lands after a flow started must not open the delete modal over the flow's popup.
func TestAppStaleAssessIgnored(t *testing.T) {
	main, feat, m := syncApp(t)
	write(t, filepath.Join(feat, ".vscode", "a.json"), "feat")
	m, assess := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m = drain(t, press(m, "s"), 100)
	m, _ = m.Update(assess())
	if c := m.(model).confirm; c != nil {
		t.Fatalf("stale assessment opened the delete modal: %+v", c)
	}
	if v := m.View(); !strings.Contains(v, "Apply these changes?") {
		t.Fatalf("flow popup hidden:\n%s", v)
	}
	m = drain(t, press(m, "n"), 100)
	if exists(filepath.Join(main, ".vscode", "a.json")) || m.(model).confirm != nil {
		t.Errorf("n applied the sync or left a modal; status %q", m.(model).statusMsg)
	}
}

// The panels and the command log, with the plan just printed, stay on screen under a flow popup.
func TestAppPopupKeepsLog(t *testing.T) {
	_, feat, m := syncApp(t)
	write(t, filepath.Join(feat, ".vscode", "a.json"), "feat")
	m = drain(t, press(m, "s"), 100)
	lines := m.(model).log.Lines()
	last := strings.TrimSpace(lines[len(lines)-1])
	last = last[:min(len(last), 20)]
	for _, sz := range [][2]int{{100, 30}, {80, 24}, {70, 30}, {60, 12}} {
		v := resize(m, sz[0], sz[1]).View()
		hasAll(t, v, "Command log", last, "Apply these changes?")
		if sz[0] >= 80 {
			hasAll(t, v, "[2] Worktrees")
		}
		checkWidth(t, v, sz[0], sz[1])
	}
}

func TestAppSyncBlockedStatus(t *testing.T) {
	main, _, m := syncApp(t)
	write(t, filepath.Join(main, ".vscode", "a.json.devex-conflict-feat"), "x")
	m = drain(t, press(m, "s"), 100)
	if s := m.(model).statusMsg; !strings.HasPrefix(s, "Sync blocked") || !m.(model).statusErr {
		t.Errorf("status %q", s)
	}
}

// A git hook line logged after the flow's own output must not become the status.
func TestAppStatusIgnoresLateLogLines(t *testing.T) {
	_, feat, m := syncApp(t)
	write(t, filepath.Join(feat, ".vscode", "a.json"), "feat")
	m = drain(t, press(m, "s"), 100)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	ch := m.(model).events
	for {
		msg := <-ch
		if _, done := msg.(flowDoneMsg); done {
			m, _ = m.Update(logMsg{"git status --porcelain\r"})
			m, _ = m.Update(msg)
			break
		}
		m, _ = m.Update(msg)
	}
	if s := m.(model).statusMsg; s != "sync: done · ✓ Synchronization complete!" {
		t.Errorf("status %q", s)
	}
}
