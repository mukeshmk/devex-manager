// Package ctx reads and edits per-branch markdown notes (todos and notes) stored under the git common dir.
// Content functions are pure string-in, string-out; callers do the file I/O.
package ctx

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Todo is one checklist item under the Todos heading. Line is the 1-based line number in the file.
type Todo struct {
	Index, Line int
	Done        bool
	Text        string
}

var (
	headingRe = regexp.MustCompile(`^##?\s+`)
	todosRe   = regexp.MustCompile(`(?i)^##\s+todos`)
	notesRe   = regexp.MustCompile(`(?i)^##\s+notes`)
	todoRe    = regexp.MustCompile(`^\s*- \[[ xX]\]\s+(.*)`)
	checkedRe = regexp.MustCompile(`\[[xX]\]`)
	doneRe    = regexp.MustCompile(`^\s*- \[[xX]\]`)
)

type section int

const (
	none section = iota
	inTodos
	inNotes
)

func split(content string) []string {
	if content == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(content, "\n"), "\n")
}

func join(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

// scan calls fn for every line with the section it belongs to; heading lines report the section they open.
func scan(lines []string, fn func(i int, line string, sec section, heading bool)) {
	sec := none
	for i, l := range lines {
		heading := headingRe.MatchString(l)
		if heading {
			switch {
			case todosRe.MatchString(l):
				sec = inTodos
			case notesRe.MatchString(l):
				sec = inNotes
			default:
				sec = none
			}
		}
		fn(i, l, sec, heading)
	}
}

// Path returns the context file for branch; slashes become "__".
func Path(commonDir, branch string) string {
	return filepath.Join(commonDir, "info", "devex", "contexts", strings.ReplaceAll(branch, "/", "__")+".md")
}

// Write replaces path atomically (temp file plus rename) so a crash never truncates notes.
func Write(path, content string) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".ctx-*")
	if err != nil {
		return err
	}
	_, werr := f.WriteString(content)
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		os.Remove(f.Name())
		return werr
	}
	if err := os.Chmod(f.Name(), 0o644); err != nil {
		os.Remove(f.Name())
		return err
	}
	if err := os.Rename(f.Name(), path); err != nil {
		os.Remove(f.Name())
		return err
	}
	return nil
}

// EnsureFile creates the file with the standard template when it does not exist.
func EnsureFile(path, branch string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return Write(path, fmt.Sprintf("# Context: %s\n\n## Todos\n\n## Notes\n", branch))
}

// ParseTodos lists todo items under the Todos heading, numbered from 1.
func ParseTodos(content string) []Todo {
	var out []Todo
	scan(split(content), func(i int, l string, sec section, heading bool) {
		if sec != inTodos || heading {
			return
		}
		if m := todoRe.FindStringSubmatch(l); m != nil {
			out = append(out, Todo{Index: len(out) + 1, Line: i + 1, Done: doneRe.MatchString(l), Text: m[1]})
		}
	})
	return out
}

// HasTodos reports whether a Todos heading exists.
func HasTodos(content string) bool { return hasHeading(content, todosRe) }

// HasNotes reports whether a Notes heading exists.
func HasNotes(content string) bool { return hasHeading(content, notesRe) }

func hasHeading(content string, re *regexp.Regexp) bool {
	for _, l := range split(content) {
		if re.MatchString(l) {
			return true
		}
	}
	return false
}

// Notes returns the lines under the Notes heading.
func Notes(content string) []string {
	var out []string
	scan(split(content), func(_ int, l string, sec section, heading bool) {
		if sec == inNotes && !heading {
			out = append(out, l)
		}
	})
	return out
}

// AddTodo inserts "- [ ] task" after the last todo of the first Todos section, creating the section if absent.
func AddTodo(content, task string) string {
	lines := split(content)
	h := -1
	for i, l := range lines {
		if todosRe.MatchString(l) {
			h = i
			break
		}
	}
	if h < 0 {
		lines = append(lines, "", "## Todos")
		h = len(lines) - 1
	}
	at := h
	for i := h + 1; i < len(lines) && !headingRe.MatchString(lines[i]); i++ {
		if todoRe.MatchString(lines[i]) {
			at = i
		}
	}
	out := append(append(append([]string{}, lines[:at+1]...), "- [ ] "+task), lines[at+1:]...)
	return join(out)
}

func find(content string, idx int) (Todo, error) {
	for _, t := range ParseTodos(content) {
		if t.Index == idx {
			return t, nil
		}
	}
	return Todo{}, fmt.Errorf("Error: Todo index %d not found.", idx)
}

// SetDone checks or unchecks todo idx.
func SetDone(content string, idx int, done bool) (string, error) {
	t, err := find(content, idx)
	if err != nil {
		return "", err
	}
	lines := split(content)
	l := lines[t.Line-1]
	if done {
		l = strings.Replace(l, "[ ]", "[x]", 1)
	} else {
		if loc := checkedRe.FindStringIndex(l); loc != nil { // first match only, like the Bash sed
			l = l[:loc[0]] + "[ ]" + l[loc[1]:]
		}
	}
	lines[t.Line-1] = l
	return join(lines), nil
}

// RemoveTodo deletes todo idx.
func RemoveTodo(content string, idx int) (string, error) {
	t, err := find(content, idx)
	if err != nil {
		return "", err
	}
	lines := split(content)
	return join(append(lines[:t.Line-1], lines[t.Line:]...)), nil
}

// ClearDone removes completed todos and returns how many.
func ClearDone(content string) (string, int) {
	var keep []string
	n := 0
	scan(split(content), func(_ int, l string, sec section, _ bool) {
		if sec == inTodos && doneRe.MatchString(l) {
			n++
			return
		}
		keep = append(keep, l)
	})
	return join(keep), n
}

// Orphans returns branch names that have a context file but no longer exist, sorted.
func Orphans(commonDir string, branchExists func(string) bool) []string {
	files, _ := filepath.Glob(filepath.Join(filepath.Dir(Path(commonDir, "x")), "*.md"))
	var out []string
	for _, f := range files {
		b := strings.ReplaceAll(strings.TrimSuffix(filepath.Base(f), ".md"), "__", "/")
		if !branchExists(b) {
			out = append(out, b)
		}
	}
	sort.Strings(out)
	return out
}
