package wtapp

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "space":
		return tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestPopupConfirmKeys(t *testing.T) {
	for k, want := range map[string]bool{"enter": true, "y": true, "n": false, "esc": false} {
		_, done, a, _ := popup{kind: popupConfirm}.update(key(k))
		if !done || a.Bool != want {
			t.Errorf("%s: done=%v bool=%v", k, done, a.Bool)
		}
	}
}

func TestPopupChooseDefaultOnEsc(t *testing.T) {
	p := popup{kind: popupChoose, options: []string{"a", "b", "c"}, def: 2}
	_, done, a, _ := p.update(key("esc"))
	if !done || a.Index != 2 {
		t.Fatalf("esc: done=%v index=%d", done, a.Index)
	}
	p, done, _, _ = p.update(key("down"))
	if done || p.cursor != 1 {
		t.Fatalf("down: done=%v cursor=%d", done, p.cursor)
	}
	_, done, a, _ = p.update(key("enter"))
	if !done || a.Index != 1 {
		t.Fatalf("enter: done=%v index=%d", done, a.Index)
	}
}

func TestPopupChecklistTicks(t *testing.T) {
	p := popup{kind: popupChecklist, options: []string{"a", "b"}, ticks: []bool{false, false}}
	p, _, _, _ = p.update(key("space"))
	if !p.ticks[0] {
		t.Fatal("space should tick")
	}
	p, _, _, _ = p.update(key("space"))
	if p.ticks[0] {
		t.Fatal("second space should untick")
	}
	p, _, _, _ = p.update(key("a"))
	_, done, a, _ := p.update(key("enter"))
	if !done || !a.Ticks[0] || !a.Ticks[1] {
		t.Fatalf("a then enter: done=%v ticks=%v", done, a.Ticks)
	}
	_, _, a, _ = p.update(key("esc"))
	if a.Ticks[0] || a.Ticks[1] {
		t.Fatalf("esc ticks=%v", a.Ticks)
	}
}

func TestPopupCtrlCAborts(t *testing.T) {
	for _, k := range []popupKind{popupConfirm, popupChoose, popupChecklist, popupInput, popupHelp} {
		p := popup{kind: k, options: []string{"a"}, ticks: []bool{false}, input: textinput.New()}
		_, done, a, _ := p.update(key("ctrl+c"))
		if !done || !a.Abort {
			t.Errorf("kind %d: done=%v abort=%v", k, done, a.Abort)
		}
	}
}

func TestPopupInputAndHelp(t *testing.T) {
	var got string
	p := popup{kind: popupInput, input: textinput.New(), onSubmit: func(s string) tea.Cmd { got = s; return nil }}
	p.input.SetValue("hi")
	if _, done, _, _ := p.update(key("enter")); !done || got != "hi" {
		t.Fatalf("input enter: done=%v got=%q", done, got)
	}
	if _, done, _, _ := (popup{kind: popupHelp}).update(key("x")); !done {
		t.Fatal("help should close on any key")
	}
}

func TestPopupViewFits(t *testing.T) {
	opts := make([]string, 20)
	for i := range opts {
		opts[i] = strings.Repeat("x", 120)
	}
	for _, k := range []popupKind{popupChoose, popupChecklist} {
		p := popup{kind: k, title: "T", options: opts, ticks: make([]bool, 20), cursor: 19}
		lines := strings.Split(p.view(60, 12), "\n")
		if len(lines) > 12 {
			t.Errorf("kind %d: %d lines", k, len(lines))
		}
		for _, l := range lines {
			if w := lipgloss.Width(l); w > 56 {
				t.Errorf("kind %d: width %d", k, w)
			}
		}
		if !strings.Contains(p.view(60, 12), "▸") {
			t.Errorf("kind %d: cursor row scrolled out of view", k)
		}
	}
}

// A long choose question is wrapped into the body, so its end stays visible at 60 columns.
func TestPopupChooseQuestionInBody(t *testing.T) {
	q := "Conflict in .vscode/some/deeply/nested/settings.json. Modified in feat and main. Which version to keep?"
	v := (popup{kind: popupChoose, title: q, options: []string{"main", "feat"}}).view(60, 20)
	if !strings.Contains(v, "Which version to keep?") || !strings.Contains(v, "Conflict in") {
		t.Errorf("question cut:\n%s", v)
	}
}
