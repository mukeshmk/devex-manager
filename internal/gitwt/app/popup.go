package wtapp

import (
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

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

// popup is a modal box. Flow questions answer through reply; input, and confirms without a reply, call onSubmit.
type popup struct {
	kind    popupKind
	title   string
	options []string // choose/checklist rows, or help lines
	cursor  int
	def     int // choose: index returned on esc
	// filterable (choose only): typing narrows options by case-insensitive substring; the first pinned options always show; cursor indexes the visible list.
	filterable bool
	filter     string
	pinned     int
	ticks      []bool
	input      textinput.Model
	reply      chan promptAnswer
	onSubmit   func(string) tea.Cmd
}

// visible returns the indices into options that are shown.
func (p popup) visible() []int {
	var idx []int
	f := strings.ToLower(p.filter)
	for i, o := range p.options {
		if i < p.pinned || strings.Contains(strings.ToLower(o), f) {
			idx = append(idx, i)
		}
	}
	return idx
}

// updateFilter handles keys of a filterable choose popup.
func (p popup) updateFilter(msg tea.KeyMsg) (popup, bool, promptAnswer, tea.Cmd) {
	edited := true
	switch k := msg.String(); {
	case k == "up":
		p.cursor = max(p.cursor-1, 0)
		return p, false, promptAnswer{}, nil
	case k == "down":
		p.cursor, edited = p.cursor+1, false
	case k == "enter":
		if vis := p.visible(); p.cursor < len(vis) {
			return p, true, promptAnswer{Index: vis[p.cursor]}, nil
		}
		return p, false, promptAnswer{}, nil
	case k == "esc" && p.filter == "":
		return p, true, promptAnswer{Index: p.def}, nil
	case k == "esc":
		p.filter = ""
	case k == "backspace":
		if r := []rune(p.filter); len(r) > 0 {
			p.filter = string(r[:len(r)-1])
		}
	case (msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace) && !msg.Alt:
		p.filter += string(msg.Runes)
	default:
		edited = false
	}
	if edited && p.filter != "" {
		p.cursor = p.pinned // land on the first real match, not the pinned row
	}
	p.cursor = max(min(p.cursor, len(p.visible())-1), 0)
	return p, false, promptAnswer{}, nil
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
			if p.reply == nil && p.onSubmit != nil { // local confirm: no flow is waiting
				return p, true, promptAnswer{Bool: true}, p.onSubmit("")
			}
			return p, true, promptAnswer{Bool: true}, nil
		case "esc", "n":
			return p, true, promptAnswer{}, nil
		}
	case popupChoose:
		if p.filterable {
			return p.updateFilter(msg)
		}
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
	// Questions go in the body, wrapped, so a long one is never cut off in the box title.
	question := strings.Split(ansi.Wrap(p.title, max(maxW-6, 1), ""), "\n")
	switch p.kind {
	case popupConfirm:
		lines, hint = question, "enter/y yes · esc/n no"
	case popupInput:
		lines, hint = []string{p.input.View()}, "enter accept · esc cancel"
	case popupHelp:
		lines = p.options
	case popupChoose:
		lines, hint = question[:min(len(question), max(maxH-7, 1))], "↑/↓ move · enter select · esc default"
		if p.filterable {
			hint = "↑/↓ move · enter select · esc clear/default"
		}
	case popupChecklist:
		hint = "space tick · a all · enter apply · esc none"
	}
	box := p.title
	switch p.kind {
	case popupConfirm:
		box = "Confirm"
	case popupChoose:
		box = "Choose"
	}
	if p.kind == popupChoose || p.kind == popupChecklist {
		idx := make([]int, len(p.options))
		for i := range idx {
			idx[i] = i
		}
		room := maxH - 4 - 1 - len(lines) // one row is the hint
		if p.filterable {
			idx = p.visible()
			lines = append(lines, "filter: "+p.filter)
			room--
			if len(idx) == 0 {
				lines = append(lines, dim.Render("no matches"))
			}
		}
		vis := max(min(len(idx), room), 1)
		off := max(p.cursor-vis+1, 0)
		for j := off; j < min(off+vis, len(idx)); j++ {
			i := idx[j]
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
