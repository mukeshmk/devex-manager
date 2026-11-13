package wtapp

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestChanPrompterRoundTrip(t *testing.T) {
	ch := make(chan tea.Msg, 4)
	cp := &chanPrompter{ch: ch}
	got := make(chan int)
	go func() { got <- cp.Choose("q", []string{"a", "b"}, 1) }()
	pm := (<-ch).(promptMsg)
	if pm.Kind != promptChoose || pm.Def != 1 {
		t.Fatalf("bad prompt %+v", pm)
	}
	pm.reply <- promptAnswer{Index: 0}
	if v := <-got; v != 0 {
		t.Fatalf("got %d", v)
	}
}

func TestChanPrompterAbortSticks(t *testing.T) {
	ch := make(chan tea.Msg, 4)
	cp := &chanPrompter{ch: ch}
	got := make(chan int)
	go func() { got <- cp.Choose("q", []string{"a", "b"}, 1) }()
	(<-ch).(promptMsg).reply <- promptAnswer{Abort: true}
	if v := <-got; v != 1 || !cp.Aborted() {
		t.Fatalf("got %d aborted=%v", v, cp.Aborted())
	}
	if cp.Confirm("again", true) || cp.Confirm("again", false) {
		t.Fatal("confirm after abort should return false")
	}
	if n := len(cp.MultiSelect("m", []string{"x", "y"}, []bool{true, true})); n != 2 {
		t.Fatalf("len %d", n)
	}
	if len(ch) != 0 {
		t.Fatal("sent after abort")
	}
}

func TestLogWriterSplitsAndStrips(t *testing.T) {
	ch := make(chan tea.Msg, 4)
	lw := &logWriter{ch: ch}
	fmt.Fprint(lw, "\x1b[31mred\x1b[0m\nhalf")
	fmt.Fprint(lw, " line\n")
	a, b := (<-ch).(logMsg), (<-ch).(logMsg)
	if a.Line != "red" || b.Line != "half line" {
		t.Fatalf("%q %q", a.Line, b.Line)
	}
}

func TestLogRingBounded(t *testing.T) {
	var r logRing
	for i := 1; i <= 600; i++ {
		r.Add(fmt.Sprint(i))
	}
	if l := r.Lines(); len(l) != 500 || l[0] != "101" {
		t.Fatalf("len %d first %s", len(l), r.Lines()[0])
	}
}
