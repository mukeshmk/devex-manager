package ui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"
)

type Prompter interface {
	Confirm(q string, def bool) bool
	Choose(q string, options []string, def int) int
	MultiSelect(q string, options []string, preselected []bool) []bool
}

// NewPrompter returns a huh-based prompter when in is a TTY, else a LinePrompter.
func NewPrompter(in *os.File, out io.Writer) Prompter {
	if IsTTY(in) {
		return huhPrompter{in: in, out: out}
	}
	return LinePrompter{R: bufio.NewReader(in), W: out}
}

// LinePrompter reads one answer per line; used for non-interactive input.
type LinePrompter struct {
	R *bufio.Reader
	W io.Writer
}

// line returns the trimmed line and false on EOF with no data.
func (p LinePrompter) line(prompt string) (string, bool) {
	fmt.Fprint(p.W, prompt)
	s, err := p.R.ReadString('\n')
	if err != nil && s == "" {
		return "", false
	}
	return strings.TrimSpace(s), true
}

func (p LinePrompter) Confirm(q string, def bool) bool {
	hint := "[y/N]"
	if def {
		hint = "[Y/n]"
	}
	s, ok := p.line(fmt.Sprintf("%s %s ", q, hint))
	if !ok {
		return false
	}
	if s == "" {
		return def
	}
	return strings.EqualFold(s, "y")
}

func (p LinePrompter) Choose(q string, options []string, def int) int {
	for i, o := range options {
		fmt.Fprintf(p.W, "  %d) %s\n", i+1, o)
	}
	s, ok := p.line(fmt.Sprintf("%s [%d] ", q, def+1))
	if !ok || s == "" {
		return def
	}
	if n, err := strconv.Atoi(s); err == nil && n >= 1 && n <= len(options) {
		return n - 1
	}
	return def
}

func (p LinePrompter) MultiSelect(q string, options []string, pre []bool) []bool {
	res := make([]bool, len(options))
	for i, o := range options {
		fmt.Fprintf(p.W, "  %d) %s\n", i+1, o)
	}
	s, ok := p.line(q + " (y=preselected, n=none, or numbers e.g. 1 3): ")
	if !ok {
		return res
	}
	switch strings.ToLower(s) {
	case "y":
		copy(res, pre)
	case "", "n":
	default:
		for _, f := range strings.Fields(s) {
			if n, err := strconv.Atoi(f); err == nil && n >= 1 && n <= len(options) {
				res[n-1] = true
			}
		}
	}
	return res
}

// DefaultPrompter always answers with the defaults (non-interactive callers).
type DefaultPrompter struct{}

func (DefaultPrompter) Confirm(_ string, def bool) bool          { return def }
func (DefaultPrompter) Choose(_ string, _ []string, def int) int { return def }
func (DefaultPrompter) MultiSelect(_ string, options []string, pre []bool) []bool {
	res := make([]bool, len(options))
	copy(res, pre)
	return res
}

type huhPrompter struct {
	in  *os.File
	out io.Writer
}

func (p huhPrompter) run(f huh.Field) error {
	return huh.NewForm(huh.NewGroup(f)).WithInput(p.in).WithOutput(p.out).Run()
}

func (p huhPrompter) Confirm(q string, def bool) bool {
	v := def
	if p.run(huh.NewConfirm().Title(q).Value(&v)) != nil {
		return false
	}
	return v
}

func (p huhPrompter) Choose(q string, options []string, def int) int {
	v := def
	opts := make([]huh.Option[int], len(options))
	for i, o := range options {
		opts[i] = huh.NewOption(o, i)
	}
	if p.run(huh.NewSelect[int]().Title(q).Options(opts...).Value(&v)) != nil {
		return def
	}
	return v
}

func (p huhPrompter) MultiSelect(q string, options []string, pre []bool) []bool {
	res := make([]bool, len(options))
	var sel []int
	opts := make([]huh.Option[int], len(options))
	for i, o := range options {
		opts[i] = huh.NewOption(o, i).Selected(i < len(pre) && pre[i])
	}
	if p.run(huh.NewMultiSelect[int]().Title(q).Options(opts...).Value(&sel)) != nil {
		return res
	}
	for _, i := range sel {
		res[i] = true
	}
	return res
}
