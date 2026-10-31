package wtapp

import (
	"slices"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/mukeshmk/devex-manager/internal/shared/ui"
)

type popupKind int

const (
	popupConfirm popupKind = iota
	popupChoose
	popupChecklist
	popupInput
	popupHelp
)

// popup is a modal box. Flow questions answer through reply; input calls onSubmit.
type popup struct {
	kind     popupKind
	title    string
	options  []string // choose/checklist rows, or help lines
	cursor   int
	def      int // choose: index returned on esc
	ticks    []bool
	input    textinput.Model
	reply    chan promptAnswer
	onSubmit func(string) tea.Cmd
}

func (p popup) update(msg tea.KeyMsg) (popup, bool, promptAnswer, tea.Cmd) {
	k := msg.String()
	if k == "ctrl+c" {
		return p, true, promptAnswer{Abort: true}, nil
	}
	switch p.kind {
	case popupConfirm:
		switch k {
		case "enter", "y":
			return p, true, promptAnswer{Bool: true}, nil
		case "esc", "n":
			return p, true, promptAnswer{}, nil
		}
	case popupChoose:
		switch k {
		case "up", "k":
			p.cursor = max(p.cursor-1, 0)
		case "down", "j":
			p.cursor = min(p.cursor+1, len(p.options)-1)
		case "enter":
			return p, true, promptAnswer{Index: p.cursor}, nil
		case "esc":
			return p, true, promptAnswer{Index: p.def}, nil
		}
	case popupChecklist:
		if len(p.ticks) != len(p.options) {
			p.ticks = make([]bool, len(p.options))
		}
		switch k {
		case "up", "k":
			p.cursor = max(p.cursor-1, 0)
		case "down", "j":
			p.cursor = min(p.cursor+1, len(p.options)-1)
		case " ", "space":
			p.ticks = slices.Clone(p.ticks)
			p.ticks[p.cursor] = !p.ticks[p.cursor]
		case "a":
			all := !slices.Contains(p.ticks, false)
			p.ticks = slices.Repeat([]bool{!all}, len(p.options))
		case "enter":
			return p, true, promptAnswer{Ticks: slices.Clone(p.ticks)}, nil
		case "esc":
			return p, true, promptAnswer{Ticks: make([]bool, len(p.options))}, nil
		}
	case popupInput:
		switch k {
		case "enter":
			if p.onSubmit != nil {
				return p, true, promptAnswer{}, p.onSubmit(p.input.Value())
			}
			return p, true, promptAnswer{}, nil
		case "esc":
			return p, true, promptAnswer{}, nil
		}
		var cmd tea.Cmd
		p.input, cmd = p.input.Update(msg)
		return p, false, promptAnswer{}, cmd
	case popupHelp:
		return p, true, promptAnswer{}, nil
	}
	return p, false, promptAnswer{}, nil
}

// view returns the bordered box; the caller centres it.
func (p popup) view(maxW, maxH int) string {
	var lines []string
	hint := ""
	switch p.kind {
	case popupConfirm:
		lines, hint = []string{p.title}, "enter/y yes · esc/n no"
	case popupInput:
		lines, hint = []string{p.input.View()}, "enter accept · esc cancel"
	case popupHelp:
		lines = p.options
	case popupChoose:
		hint = "↑/↓ move · enter select · esc default"
	case popupChecklist:
		hint = "space tick · a all · enter apply · esc none"
	}
	box := p.title
	if p.kind == popupConfirm {
		box = "Confirm"
	}
	if p.kind == popupChoose || p.kind == popupChecklist {
		vis := max(min(len(p.options), maxH-4-1), 1) // one row is the hint
		off := max(p.cursor-vis+1, 0)
		for i := off; i < min(off+vis, len(p.options)); i++ {
			mark, row := "  ", p.options[i]
			if p.kind == popupChecklist {
				row = "[ ] " + row
				if i < len(p.ticks) && p.ticks[i] {
					row = "[x] " + row[4:]
				}
			}
			if i == p.cursor {
				mark = "▸ "
			}
			lines = append(lines, mark+row)
		}
	} else if keep := max(maxH-4-1, 1); len(lines) > keep {
		lines = lines[:keep]
	}
	if hint != "" {
		lines = append(lines, dim.Render(hint))
	}
	longest := lipgloss.Width(box)
	for _, l := range lines {
		longest = max(longest, lipgloss.Width(l))
	}
	w := min(maxW-4, max(40, longest+4))
	return ui.Box(box, lines, w, len(lines)+2, accent)
}
