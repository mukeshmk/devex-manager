// Package flows holds the sync, clean and rm flows shared by the git wt commands and the full-screen app.
package flows

import (
	"errors"
	"io"

	"github.com/charmbracelet/lipgloss"

	"github.com/mukeshmk/devex-manager/internal/shared/ui"
)

// Env is where a flow runs and how it talks to the user.
type Env struct {
	Cwd      string
	Out, Err io.Writer
	Prompter ui.Prompter
	TTY      bool // may ask questions
}

// ErrReported means the flow already printed its message; the caller just exits 1.
var ErrReported = errors.New("reported")

type palette struct{ red, green, yellow, blue lipgloss.Style }

func colours(w io.Writer) palette {
	r := lipgloss.NewRenderer(w) // plain text unless w is a colour terminal
	st := func(c string) lipgloss.Style { return r.NewStyle().Foreground(lipgloss.Color(c)) }
	return palette{st("1"), st("2"), st("11"), st("4")}
}
