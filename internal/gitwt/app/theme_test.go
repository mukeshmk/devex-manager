package wtapp

import (
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/mukeshmk/devex-manager/internal/shared/config"
)

// withColour forces a colour profile so styles render escapes, restoring the profile and default theme after.
func withColour(t *testing.T) {
	t.Helper()
	old := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI256)
	t.Cleanup(func() {
		lipgloss.SetColorProfile(old)
		SetTheme(config.DefaultTheme())
	})
}

// lineWith returns the first line of v containing s.
func lineWith(t *testing.T, v, s string) string {
	t.Helper()
	for _, l := range strings.Split(v, "\n") {
		if strings.Contains(l, s) {
			return l
		}
	}
	t.Fatalf("no line with %q:\n%s", s, v)
	return ""
}

func TestParseSpec(t *testing.T) {
	withColour(t)
	ok := map[string]string{
		"green, bold":  lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true).Render("x"),
		"default":      "x",
		"#ff0000":      lipgloss.NewStyle().Foreground(lipgloss.Color("#ff0000")).Render("x"),
		"196":          lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("x"),
		"Blue , BOLD":  lipgloss.NewStyle().Foreground(lipgloss.Color("4")).Bold(true).Render("x"),
		"red, reverse": lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Reverse(true).Render("x"),
	}
	for spec, want := range ok {
		st, err := parseSpec(spec, false)
		if err != nil || st.Render("x") != want {
			t.Errorf("%q: %q %v, want %q", spec, st.Render("x"), err, want)
		}
	}
	for _, spec := range []string{"blu", "300", "#12", "red, blue", ""} {
		if _, err := parseSpec(spec, false); err == nil {
			t.Errorf("%q: no error", spec)
		}
	}
	if st, _ := parseSpec("blue", true); st.Render("x") != lipgloss.NewStyle().Background(lipgloss.Color("4")).Render("x") {
		t.Errorf("bg: %q", st.Render("x"))
	}
}

func TestSetThemeWarns(t *testing.T) {
	withColour(t)
	th := config.DefaultTheme()
	th.ActiveBorder = "blu"
	if w := SetTheme(th); w != `theme: unknown colour "blu" in active_border_color, using default` {
		t.Fatal(w)
	}
	if active.Render("x") != lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true).Render("x") {
		t.Errorf("active not default: %q", active.Render("x"))
	}
}

func wide(t *testing.T) tea.Model {
	_, m := start(t)
	return resize(m, 120, 40)
}

func TestFocusedBorderGreenBold(t *testing.T) {
	withColour(t)
	if l := lineWith(t, wide(t).View(), "[2] Worktrees"); !strings.Contains(l, active.Render("╭─")) {
		t.Errorf("not active: %q", l)
	}
}

func TestUnfocusedBorderDefault(t *testing.T) {
	withColour(t)
	if l := lineWith(t, wide(t).View(), "[3] Context"); strings.Contains(l, active.Render("╭─")) {
		t.Errorf("unfocused uses active: %q", l)
	}
}

func TestFilterBorderCyan(t *testing.T) {
	withColour(t)
	m := press(wide(t), "/", "x")
	if l := lineWith(t, m.View(), "[2] Worktrees"); !strings.Contains(l, searching.Render("╭─")) {
		t.Errorf("not searching: %q", l)
	}
}

func TestHintLineBlue(t *testing.T) {
	withColour(t)
	v := wide(t).View()
	if l := v[strings.LastIndex(v, "\n")+1:]; !strings.Contains(l, "\x1b[34m") {
		t.Errorf("hint not blue: %q", l)
	}
}

func TestPopupBorderGreen(t *testing.T) {
	withColour(t)
	m := press(wide(t), "?")
	if l := lineWith(t, m.View(), " Keys "); !strings.Contains(l, active.Render("╭─")) {
		t.Errorf("popup not active: %q", l)
	}
}

func TestThemeFromConfig(t *testing.T) {
	withColour(t)
	th := config.DefaultTheme()
	th.ActiveBorder = "magenta"
	SetTheme(th)
	want := lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Render("╭─")
	if l := lineWith(t, wide(t).View(), "[2] Worktrees"); !strings.Contains(l, want) {
		t.Errorf("not magenta: %q", l)
	}
}

func TestThemeWarningInStatus(t *testing.T) {
	withColour(t)
	th := config.DefaultTheme()
	th.OptionsText = "nope"
	SetTheme(th)
	if m := New(nil, config.RepoConfig{}, "").(model); !strings.Contains(m.statusMsg, "theme: unknown colour") {
		t.Errorf("status %q", m.statusMsg)
	}
}

// bgLines returns the lines of v that set background SGR code (e.g. "44").
func bgLines(v, code string) []string {
	re := regexp.MustCompile(`\x1b\[([0-9;]*;)?` + code + `(;[0-9;]*)?m`)
	var out []string
	for _, l := range strings.Split(v, "\n") {
		if re.MatchString(l) {
			out = append(out, l)
		}
	}
	return out
}

func TestSelectedRowHighlightFollowsFocus(t *testing.T) {
	withColour(t)
	f, m := start(t)
	ctxFile(t, f, "- [ ] one\n")
	m = press(resize(m, 120, 40), "/", "DEVX-2", "enter")
	if ls := bgLines(m.View(), "44"); len(ls) != 1 || !strings.Contains(ls[0], "DEVX-2") {
		t.Errorf("want only the worktree row highlighted: %q", ls)
	}
	m = press(m, "3")
	if ls := bgLines(m.View(), "44"); len(ls) != 1 || !strings.Contains(ls[0], "one") {
		t.Errorf("want only the todo row highlighted: %q", ls)
	}
}

func TestPopupCursorHighlighted(t *testing.T) {
	withColour(t)
	if ls := bgLines(filterPopup("a", "b", "c").view(60, 12), "44"); len(ls) != 1 {
		t.Errorf("%d highlighted rows: %q", len(ls), ls)
	}
}

func TestSelectedBgFromConfig(t *testing.T) {
	withColour(t)
	th := config.DefaultTheme()
	th.SelectedLineBg = "magenta"
	SetTheme(th)
	m := press(wide(t), "/", "DEVX-2", "enter")
	if ls := bgLines(m.View(), "45"); len(ls) != 1 {
		t.Errorf("not magenta: %q", ls)
	}
}
