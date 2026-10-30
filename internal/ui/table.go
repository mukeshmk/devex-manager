package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/charmbracelet/x/ansi"
)

// Column describes a table column. Flex columns absorb the remaining width
// (and are truncated with "…"); other columns take their content width.
type Column struct {
	Title string
	Min   int
	Flex  bool
}

// Table renders a rounded table that never exceeds width visible cells per line.
func Table(cols []Column, rows [][]string, width int) string {
	n := len(cols)
	if n == 0 {
		return ""
	}
	nat := make([]int, n)
	for i, c := range cols {
		nat[i] = lipgloss.Width(c.Title)
		for _, r := range rows {
			if i < len(r) {
				nat[i] = max(nat[i], lipgloss.Width(r[i]))
			}
		}
	}
	// borders (n+1) plus one space of padding on each side of every column
	avail := width - (n + 1) - 2*n
	sum := 0
	for _, w := range nat {
		sum += w
	}
	w := append([]int(nil), nat...)
	if sum > avail {
		fixed, flex := 0, 0
		for i, c := range cols {
			if c.Flex {
				flex++
			} else {
				fixed += nat[i]
			}
		}
		if flex > 0 {
			share := max((avail-fixed)/flex, 1)
			for i, c := range cols {
				if c.Flex {
					w[i] = min(nat[i], max(share, c.Min))
				}
			}
		}
		// Still too wide: shrink the widest column above its Min, then above 1.
		for floor := 0; floor < 2; floor++ {
			for total(w) > avail {
				k := -1
				for i, c := range cols {
					lo := 1
					if floor == 0 {
						lo = c.Min
					}
					if w[i] > lo && (k < 0 || w[i] > w[k]) {
						k = i
					}
				}
				if k < 0 {
					break
				}
				w[k]--
			}
		}
	}

	cell := func(s string, i int) string {
		s = strings.ReplaceAll(s, "\n", " ")
		if lipgloss.Width(s) > w[i] {
			return ansi.Truncate(s, w[i], "…")
		}
		return s
	}
	heads := make([]string, n)
	for i, c := range cols {
		heads[i] = cell(c.Title, i)
	}
	t := table.New().Border(lipgloss.RoundedBorder()).Headers(heads...).
		StyleFunc(func(_, col int) lipgloss.Style {
			return lipgloss.NewStyle().Padding(0, 1).Width(w[col] + 2)
		})
	for _, r := range rows {
		out := make([]string, n)
		for i := range out {
			if i < len(r) {
				out[i] = cell(r[i], i)
			}
		}
		t.Row(out...)
	}
	lines := strings.Split(t.Render(), "\n")
	for i, l := range lines { // last resort when even 1-cell columns cannot fit
		if lipgloss.Width(l) > width {
			lines[i] = ansi.Truncate(l, width, "")
		}
	}
	return strings.Join(lines, "\n")
}

func total(w []int) int {
	s := 0
	for _, v := range w {
		s += v
	}
	return s
}
