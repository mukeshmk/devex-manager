package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func profile(t *testing.T, p termenv.Profile) {
	t.Helper()
	old := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(p)
	t.Cleanup(func() { lipgloss.SetColorProfile(old) })
}

var hlBg = lipgloss.NewStyle().Background(lipgloss.Color("4"))

func TestHighlightWidth(t *testing.T) {
	profile(t, termenv.ANSI256)
	for _, s := range []string{"abc", strings.Repeat("界", 30), "a\x1b[33mmerged\x1b[0m b"} {
		for _, w := range []int{1, 10, 40} {
			if got := lipgloss.Width(Highlight(s, w, hlBg)); got != w {
				t.Errorf("%q w=%d: width %d", s, w, got)
			}
		}
	}
}

func TestHighlightKeepsBackgroundAfterReset(t *testing.T) {
	profile(t, termenv.ANSI256)
	s := "x " + lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Render("merged") + " y"
	out := Highlight(s, 20, hlBg)
	open := hlBg.Render("x")[:strings.Index(hlBg.Render("x"), "x")]
	parts := strings.Split(out, "\x1b[0m")
	for i, p := range parts[1 : len(parts)-1] {
		if !strings.HasPrefix(p, open) {
			t.Errorf("reset %d not followed by %q: %q", i, open, out)
		}
	}
	if !strings.Contains(out, "33") {
		t.Errorf("lost yellow: %q", out)
	}
}

func TestHighlightNoColour(t *testing.T) {
	profile(t, termenv.Ascii)
	s := "a\x1b[33mmerged\x1b[0m b"
	if got := Highlight(s, 10, hlBg); got != Fit(s, 10) {
		t.Errorf("%q != %q", got, Fit(s, 10))
	}
	if got := Highlight("abc", 10, lipgloss.NewStyle()); got != Fit("abc", 10) {
		t.Errorf("empty style: %q", got)
	}
}
