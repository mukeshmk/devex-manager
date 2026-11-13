package wtapp

import (
	"bytes"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/mukeshmk/devex-manager/internal/gitwt/flows"
)

// Messages a flow goroutine sends to the model over the events channel.
type (
	promptKind int

	promptMsg struct {
		Kind     promptKind
		Question string
		Options  []string
		Def      int
		Pre      []bool
		reply    chan promptAnswer
	}
	promptAnswer struct {
		Bool  bool
		Index int
		Ticks []bool
		Abort bool
	}
	logMsg struct{ Line string }
	// flowDoneMsg carries the flow's own first error line and last output line, so the status never picks up a stray log line.
	flowDoneMsg struct {
		Name              string
		Err               error
		FirstErr, LastOut string
	}
)

const (
	promptConfirm promptKind = iota
	promptChoose
	promptChecklist
)

// waitEvent returns the next message on ch; the model re-issues it after every event.
func waitEvent(ch chan tea.Msg) tea.Cmd { return func() tea.Msg { return <-ch } }

// chanPrompter implements ui.Prompter by asking the model to show a popup.
type chanPrompter struct {
	ch      chan tea.Msg
	aborted bool
}

func (p *chanPrompter) ask(m promptMsg) promptAnswer {
	m.reply = make(chan promptAnswer, 1)
	p.ch <- m
	a := <-m.reply
	if a.Abort {
		p.aborted = true
	}
	return a
}

func (p *chanPrompter) Confirm(q string, def bool) bool {
	if p.aborted {
		return false
	}
	d := 0
	if def {
		d = 1
	}
	a := p.ask(promptMsg{Kind: promptConfirm, Question: q, Def: d})
	return !a.Abort && a.Bool
}

func (p *chanPrompter) Choose(q string, options []string, def int) int {
	if p.aborted {
		return def
	}
	a := p.ask(promptMsg{Kind: promptChoose, Question: q, Options: options, Def: def})
	if a.Abort {
		return def
	}
	return a.Index
}

func (p *chanPrompter) MultiSelect(q string, options []string, pre []bool) []bool {
	if p.aborted {
		return make([]bool, len(options))
	}
	a := p.ask(promptMsg{Kind: promptChecklist, Question: q, Options: options, Pre: pre})
	if a.Abort || len(a.Ticks) != len(options) {
		return make([]bool, len(options))
	}
	return a.Ticks
}

func (p *chanPrompter) Aborted() bool { return p.aborted }

// logWriter turns written text into one logMsg per line. Lines are dropped when the channel is full so a flow never stalls on logging.
type logWriter struct {
	ch          chan tea.Msg
	buf         []byte
	first, last string // first and last non-empty lines, trimmed
}

func (w *logWriter) Write(b []byte) (int, error) {
	w.buf = append(w.buf, b...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			return len(b), nil
		}
		w.send(string(w.buf[:i]))
		w.buf = w.buf[i+1:]
	}
}

func (w *logWriter) send(line string) {
	line = ansi.Strip(line)
	if t := strings.TrimSpace(line); t != "" {
		if w.first == "" {
			w.first = t
		}
		w.last = t
	}
	select {
	case w.ch <- logMsg{line}:
	default:
	}
}

// flush emits a held partial last line.
func (w *logWriter) flush() {
	if len(w.buf) > 0 {
		w.send(string(w.buf))
		w.buf = nil
	}
}

// runFlow runs fn in a goroutine with an Env wired to ch, then sends flowDoneMsg.
func runFlow(ch chan tea.Msg, name string, fn func(flows.Env) error, cwd string) tea.Cmd {
	return func() tea.Msg {
		go func() {
			out, errw := &logWriter{ch: ch}, &logWriter{ch: ch}
			err := fn(flows.Env{Cwd: cwd, Out: out, Err: errw, Prompter: &chanPrompter{ch: ch}, TTY: true})
			out.flush()
			errw.flush()
			ch <- flowDoneMsg{name, err, errw.first, out.last}
		}()
		return nil
	}
}

const logRingMax = 500

// logRing keeps the most recent log lines.
type logRing struct{ lines []string }

func (r *logRing) Add(line string) {
	r.lines = append(r.lines, line)
	if len(r.lines) > logRingMax {
		r.lines = r.lines[len(r.lines)-logRingMax:]
	}
}

func (r logRing) Lines() []string { return r.lines }
