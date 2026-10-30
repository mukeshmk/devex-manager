package skillsapp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/mukeshmk/devex-manager/internal/config"
)

var t0 = time.Date(2026, 5, 27, 21, 0, 0, 0, time.Local)

func fixture(t *testing.T) config.SkillsConfig {
	root := t.TempDir()
	cfg := config.SkillsConfig{MasterDir: filepath.Join(root, "master")}
	for _, n := range []string{"claude", "kiro"} {
		cfg.Tools = append(cfg.Tools, config.SkillTool{Name: n, Dir: filepath.Join(root, n)})
	}
	return cfg
}

func skill(t *testing.T, dir, name, body string, mt time.Time) {
	t.Helper()
	p := filepath.Join(dir, name, "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	os.Chtimes(p, mt, mt)
}

// send feeds msg to m and runs any resulting commands synchronously (batches included).
func send(m tea.Model, msg tea.Msg) tea.Model {
	m, cmd := m.Update(msg)
	return drain(m, cmd)
}

func drain(m tea.Model, cmd tea.Cmd) tea.Model {
	if cmd == nil {
		return m
	}
	switch msg := cmd().(type) {
	case nil:
	case tea.QuitMsg:
	case tea.BatchMsg:
		for _, c := range msg {
			m = drain(m, c)
		}
	default:
		m = send(m, msg)
	}
	return m
}

func start(t *testing.T, cfg config.SkillsConfig, w, h int) tea.Model {
	m := New(cfg)
	m = drain(m, m.Init())
	return send(m, tea.WindowSizeMsg{Width: w, Height: h})
}

func key(s string) tea.KeyMsg {
	switch s {
	case "space":
		return tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func line(t *testing.T, v, sub string) string {
	t.Helper()
	for _, l := range strings.Split(ansi.Strip(v), "\n") {
		if strings.HasPrefix(l, "│") && strings.Contains(l, sub) { // grid rows, not titles
			return l
		}
	}
	t.Fatalf("no line with %q:\n%s", sub, v)
	return ""
}

func marks(l string) string {
	var b strings.Builder
	for _, r := range l {
		if strings.ContainsRune("★≠✓·", r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func TestGridMarks(t *testing.T) {
	cfg := fixture(t)
	skill(t, cfg.MasterDir, "alpha", "old\n", t0)
	skill(t, cfg.Tools[0].Dir, "alpha", "new\n", t0.Add(time.Hour))
	for _, d := range []string{cfg.MasterDir, cfg.Tools[0].Dir, cfg.Tools[1].Dir} {
		skill(t, d, "beta", "same\n", t0)
	}
	v := start(t, cfg, 120, 20).View()
	if got := marks(line(t, v, "alpha")); got != "≠★·" {
		t.Errorf("alpha marks %q, want ≠★· (master claude kiro):\n%s", got, v)
	}
	if got := marks(line(t, v, "beta")); got != "★✓✓" {
		t.Errorf("beta marks %q, want ★✓✓:\n%s", got, v)
	}
	if l := line(t, v, "alpha"); !strings.Contains(l, "[x]") {
		t.Errorf("alpha pending should be ticked: %q", l)
	}
	if l := line(t, v, "beta"); strings.Contains(l, "[") {
		t.Errorf("beta has no pending updates, no tick box: %q", l)
	}
	// diff pane: newest (claude) vs first differing copy (master)
	if !strings.Contains(v, "-new") || !strings.Contains(v, "+old") {
		t.Errorf("diff pane missing:\n%s", v)
	}
}

func read(t *testing.T, p string) string {
	b, _ := os.ReadFile(p)
	return string(b)
}

func TestGridUntickSkipsApply(t *testing.T) {
	cfg := fixture(t)
	skill(t, cfg.MasterDir, "alpha", "a1\n", t0)
	skill(t, cfg.Tools[0].Dir, "alpha", "a2\n", t0.Add(time.Hour))
	skill(t, cfg.MasterDir, "beta", "b1\n", t0)
	skill(t, cfg.Tools[0].Dir, "beta", "b2\n", t0.Add(time.Hour))
	m := start(t, cfg, 120, 20)
	m = send(m, key("space")) // untick alpha (selected first)
	if l := line(t, m.View(), "alpha"); !strings.Contains(l, "[ ]") {
		t.Fatalf("alpha not unticked: %q", l)
	}
	m = send(m, key("a"))
	if got := read(t, filepath.Join(cfg.MasterDir, "alpha", "SKILL.md")); got != "a1\n" {
		t.Errorf("unticked alpha was synced: %q", got)
	}
	for _, d := range []string{cfg.MasterDir, cfg.Tools[1].Dir} {
		if got := read(t, filepath.Join(d, "beta", "SKILL.md")); got != "b2\n" {
			t.Errorf("beta not synced into %s: %q", d, got)
		}
	}
	v := m.View()
	if !strings.Contains(v, "Applied 2 update(s)") || !strings.Contains(v, "Backups stored in:") {
		t.Errorf("status missing:\n%s", v)
	}
	if got := marks(line(t, v, "beta")); got != "★✓✓" { // rescanned; copies keep mtime so the tie goes to master
		t.Errorf("beta after apply %q:\n%s", got, v)
	}
	if l := line(t, v, "alpha"); !strings.Contains(l, "[ ]") {
		t.Errorf("untick lost on rescan: %q", l)
	}
}

func TestGridResize(t *testing.T) {
	cfg := fixture(t)
	for i, n := range []string{"alpha", "a-very-long-skill-name-that-will-not-fit-anywhere"} {
		skill(t, cfg.MasterDir, n, strings.Repeat("x", 300)+"\n\tTab\n", t0)
		skill(t, cfg.Tools[i].Dir, n, "other\n", t0.Add(time.Hour))
	}
	m := start(t, cfg, 200, 30)
	for _, size := range [][2]int{{80, 24}, {200, 30}, {60, 12}} {
		m = send(m, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		v := m.View()
		ls := strings.Split(v, "\n")
		if len(ls) != size[1] {
			t.Errorf("%v: %d lines", size, len(ls))
		}
		for _, l := range ls {
			if w := lipgloss.Width(l); w > size[0] {
				t.Errorf("%v: line width %d: %q", size, w, l)
			}
		}
	}
	m = send(m, tea.KeyMsg{Type: tea.KeyPgDown})
	m = send(m, key("/"))
	for _, r := range "long" {
		m = send(m, key(string(r)))
	}
	m = send(m, key("enter"))
	if v := ansi.Strip(m.View()); strings.Contains(v, "alpha") {
		t.Errorf("filter kept alpha:\n%s", v)
	}
}

func TestGridTooSmall(t *testing.T) {
	m := start(t, fixture(t), 59, 30)
	if v := m.View(); v != "Terminal too small (need 60×12)" {
		t.Errorf("got %q", v)
	}
	m = send(m, tea.WindowSizeMsg{Width: 100, Height: 11})
	if v := m.View(); v != "Terminal too small (need 60×12)" {
		t.Errorf("got %q", v)
	}
}
