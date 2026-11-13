package ui

import (
	"os"
	"strconv"

	"golang.org/x/term"
)

// IsTTY reports whether f is an interactive terminal.
func IsTTY(f *os.File) bool {
	return f != nil && term.IsTerminal(int(f.Fd()))
}

// Width returns the terminal width of f, falling back to $COLUMNS, then 100.
func Width(f *os.File) int {
	if f != nil {
		if w, _, err := term.GetSize(int(f.Fd())); err == nil && w > 0 {
			return w
		}
	}
	if w, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil && w > 0 {
		return w
	}
	return 100
}
