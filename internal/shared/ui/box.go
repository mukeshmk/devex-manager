package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

var bold = lipgloss.NewStyle().Bold(true)

// Fit truncates s to w cells with … and pads it to exactly w.
func Fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	s = ansi.Truncate(s, w, "…")
	return s + strings.Repeat(" ", w-lipgloss.Width(s))
}

// Box draws a rounded border of exactly w×h in border's style, with a bold title in the top edge.
func Box(title string, lines []string, w, h int, border lipgloss.Style) string {
	in := w - 2
	title = ansi.Truncate(" "+title+" ", max(in-1, 0), "…")
	out := []string{border.Render("╭─") + bold.Render(title) + border.Render(strings.Repeat("─", max(in-1-lipgloss.Width(title), 0))+"╮")}
	for i := 0; i < h-2; i++ {
		l := ""
		if i < len(lines) {
			l = lines[i]
		}
		out = append(out, border.Render("│")+Fit(l, in)+border.Render("│"))
	}
	return strings.Join(append(out, border.Render("╰"+strings.Repeat("─", in)+"╯")), "\n")
}

// Highlight fits s to exactly w cells and paints the whole row in st, reopening st after each reset inside s
// so a background runs to the end of the row while inner colours survive.
func Highlight(s string, w int, st lipgloss.Style) string {
	s = Fit(s, w)
	open, _, _ := strings.Cut(st.Render("x"), "x")
	if open == "" { // empty style or no-colour terminal
		return s
	}
	s = strings.NewReplacer("\x1b[0m", "\x1b[0m"+open, "\x1b[m", "\x1b[m"+open).Replace(s)
	return open + s + "\x1b[0m"
}
