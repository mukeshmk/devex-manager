package ui

import (
	"bufio"
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

var cols = []Column{
	{Title: "Worktree", Min: 8},
	{Title: "Branch", Min: 10, Flex: true},
	{Title: "Status", Min: 6},
	{Title: "Ahead", Min: 5},
	{Title: "Behind", Min: 6},
	{Title: "Path", Min: 8},
}

func maxLineWidth(s string) int {
	m := 0
	for _, l := range strings.Split(s, "\n") {
		if w := lipgloss.Width(l); w > m {
			m = w
		}
	}
	return m
}

func TestTableFitsWidth(t *testing.T) {
	branch := "feature/" + strings.Repeat("x", 52)
	rows := [][]string{{"main", branch, "clean", "1", "2", "/tmp/a"}}
	for _, w := range []int{60, 90, 160} {
		out := Table(cols, rows, w)
		for _, l := range strings.Split(out, "\n") {
			if lipgloss.Width(l) > w {
				t.Fatalf("width %d: line %q too wide", w, l)
			}
		}
		if w == 160 && !strings.Contains(out, branch) {
			t.Fatalf("full branch missing at 160:\n%s", out)
		}
		if w == 90 && !strings.Contains(out, "…") {
			t.Fatalf("expected ellipsis at 90:\n%s", out)
		}
	}
}

func TestTableColouredCellsAlign(t *testing.T) {
	red := lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
	rows := [][]string{
		{red.Render("main"), "a", red.Render("clean"), "1", "2", "/p"},
		{"longer-name", red.Render("b"), "dirty", "10", "20", "/path"},
	}
	lines := strings.Split(Table(cols, rows, 100), "\n")
	w := lipgloss.Width(lines[0])
	for _, l := range lines {
		if lipgloss.Width(l) != w {
			t.Fatalf("misaligned: %d vs %d in %q", lipgloss.Width(l), w, l)
		}
	}
}

func TestTableNarrowNoPanic(t *testing.T) {
	out := Table(cols, [][]string{{"main", "feature/long-branch", "clean", "1", "2", "/tmp/a"}}, 20)
	if m := maxLineWidth(out); m > 20 {
		t.Fatalf("width %d > 20:\n%s", m, out)
	}
}

func TestLinePrompterEOF(t *testing.T) {
	p := LinePrompter{R: bufio.NewReader(strings.NewReader("")), W: &bytes.Buffer{}}
	if p.Confirm("ok?", true) {
		t.Fatal("EOF Confirm must be false")
	}
	if p.Choose("pick", []string{"a", "b"}, 1) != 1 {
		t.Fatal("EOF Choose must be def")
	}
}

func TestLinePrompterMulti(t *testing.T) {
	p := LinePrompter{R: bufio.NewReader(strings.NewReader("1 3\n")), W: &bytes.Buffer{}}
	got := p.MultiSelect("pick", []string{"a", "b", "c"}, nil)
	if !reflect.DeepEqual(got, []bool{true, false, true}) {
		t.Fatalf("got %v", got)
	}
}

func TestLinePrompterMisc(t *testing.T) {
	p := LinePrompter{R: bufio.NewReader(strings.NewReader("y\n\n2\nx\ny\n")), W: &bytes.Buffer{}}
	if !p.Confirm("q", false) {
		t.Fatal("y")
	}
	if !p.Confirm("q", true) {
		t.Fatal("empty -> def")
	}
	if p.Choose("q", []string{"a", "b"}, 0) != 1 {
		t.Fatal("choose 2")
	}
	if p.Choose("q", []string{"a", "b"}, 0) != 0 {
		t.Fatal("invalid -> def")
	}
	if got := p.MultiSelect("q", []string{"a", "b"}, []bool{false, true}); !reflect.DeepEqual(got, []bool{false, true}) {
		t.Fatalf("y -> preselected, got %v", got)
	}
}

func TestDefaultPrompter(t *testing.T) {
	var p Prompter = DefaultPrompter{}
	if !p.Confirm("q", true) || p.Choose("q", []string{"a", "b"}, 1) != 1 {
		t.Fatal("defaults")
	}
	if got := p.MultiSelect("q", []string{"a", "b"}, []bool{true, false}); !reflect.DeepEqual(got, []bool{true, false}) {
		t.Fatal(got)
	}
}
