// Package wtapp is the full-screen `git wt` app: worktree list, details pane, single-key actions.
package wtapp

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/mukeshmk/devex-manager/internal/config"
	"github.com/mukeshmk/devex-manager/internal/ctx"
	"github.com/mukeshmk/devex-manager/internal/gitx"
	"github.com/mukeshmk/devex-manager/internal/wt"
	"github.com/mukeshmk/devex-manager/internal/wtsync"
)

const (
	minW, minH = 60, 12
	tooSmall   = "Terminal too small (need 60×12)"
	maxLost    = 10
)

var helpItems = []string{"↑↓ move", "tab todos", "space toggle", "/ filter", "s sync", "d delete", "c clean", "r refresh", "q quit"}

type (
	loadedMsg struct {
		rows []wt.Row
		err  error
	}
	assessedMsg struct {
		confirm
		err error
	}
	execDoneMsg struct {
		args []string
		err  error
	}
)

type confirm struct {
	folder string
	safety wt.Safety
	refuse string // non-empty: git wt rm would refuse this outright, so the modal only offers close
}

type model struct {
	repo *gitx.Repo
	cfg  config.RepoConfig
	self string
	exec func(*exec.Cmd) tea.Cmd // injectable so tests can see argv without running anything

	w, h               int
	rows               []wt.Row
	loaded             bool
	err                error
	sel, todoSel       int
	filter             string
	filtering, inTodos bool
	confirm            *confirm
	status             string
	statusErr          bool
}

// New builds the app; self is the path of the running binary, re-invoked as git-wt for actions.
func New(repo *gitx.Repo, cfg config.RepoConfig, self string) tea.Model {
	return model{repo: repo, cfg: cfg, self: self, exec: func(c *exec.Cmd) tea.Cmd {
		return tea.Exec(&paused{Cmd: c}, func(err error) tea.Msg { return execDoneMsg{c.Args, err} })
	}}
}

// paused runs the child, then waits for Enter so its output is read before the alt screen returns.
type paused struct{ *exec.Cmd }

func (p *paused) SetStdin(r io.Reader)  { p.Stdin = r }
func (p *paused) SetStdout(w io.Writer) { p.Stdout = w }
func (p *paused) SetStderr(w io.Writer) { p.Stderr = w }

func (p *paused) Run() error {
	err := p.Cmd.Run()
	fmt.Fprint(p.Stdout, "\nPress Enter to return to git wt…")
	bufio.NewReader(p.Stdin).ReadString('\n')
	return err
}

// Run shows m full-screen until the user quits.
func Run(m tea.Model) error {
	_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

func (m model) Init() tea.Cmd { return m.load() }

func (m model) load() tea.Cmd {
	repo, cfg := m.repo, m.cfg
	return func() tea.Msg {
		rows, err := wt.Collect(repo, cfg)
		return loadedMsg{rows, err}
	}
}

func (m model) run(args ...string) tea.Cmd {
	return m.exec(&exec.Cmd{Path: m.self, Args: append([]string{"git-wt"}, args...)})
}

// visible is the filtered row list; sel indexes into it.
func (m model) visible() []wt.Row {
	if m.filter == "" {
		return m.rows
	}
	var out []wt.Row
	for _, r := range m.rows {
		if strings.Contains(r.Folder, m.filter) || strings.Contains(r.WT.Branch, m.filter) {
			out = append(out, r)
		}
	}
	return out
}

func (m model) current() (wt.Row, bool) {
	v := m.visible()
	if m.sel < 0 || m.sel >= len(v) {
		return wt.Row{}, false
	}
	return v[m.sel], true
}

// todos reads the selected branch's ctx file; detached or missing file means no todos.
func (m model) todos() (path, content string, todos []ctx.Todo) {
	r, ok := m.current()
	if !ok || r.WT.Branch == "" || r.WT.Detached {
		return "", "", nil
	}
	path = ctx.Path(m.repo.CommonDir, r.WT.Branch)
	b, err := os.ReadFile(path)
	if err != nil {
		return "", "", nil
	}
	return path, string(b), ctx.ParseTodos(string(b))
}

func (m model) clamp() model {
	if n := len(m.visible()); m.sel >= n {
		m.sel = n - 1
	}
	if m.sel < 0 {
		m.sel = 0
	}
	_, _, t := m.todos()
	if m.todoSel >= len(t) {
		m.todoSel = len(t) - 1
	}
	if m.todoSel < 0 {
		m.todoSel = 0
	}
	return m
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
	case loadedMsg:
		m.rows, m.err, m.loaded = msg.rows, msg.err, true
		m = m.clamp()
	case assessedMsg:
		if msg.err != nil {
			m.status, m.statusErr = "Error: "+msg.err.Error(), true
		} else {
			m.confirm = &msg.confirm
		}
	case execDoneMsg:
		// The child's output vanishes with the alt screen, so say how it ended.
		m.status, m.statusErr = strings.Join(msg.args, " ")+": done", msg.err != nil
		if msg.err != nil {
			m.status = fmt.Sprintf("%s: %v", strings.Join(msg.args, " "), msg.err)
		}
		return m, m.load()
	case tea.KeyMsg:
		return m.key(msg)
	}
	return m, nil
}

func (m model) key(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := msg.String()
	if k == "ctrl+c" {
		return m, tea.Quit
	}
	m.status = ""
	if m.confirm != nil {
		switch k {
		case "y":
			if !m.confirm.canDelete() {
				return m, nil
			}
			f := m.confirm.folder
			m.confirm = nil
			return m, m.run("rm", f)
		case "n", "esc":
			m.confirm = nil
		}
		return m, nil
	}
	if m.filtering {
		switch msg.Type {
		case tea.KeyEnter:
			m.filtering = false
		case tea.KeyEsc:
			m.filtering, m.filter = false, ""
		case tea.KeyBackspace:
			if r := []rune(m.filter); len(r) > 0 {
				m.filter = string(r[:len(r)-1])
			}
		case tea.KeyRunes, tea.KeySpace:
			m.filter += string(msg.Runes) // KeySpace carries its ' ' rune
		}
		m.sel = 0
		return m.clamp(), nil
	}
	switch k {
	case "q":
		return m, tea.Quit
	case "esc":
		m.inTodos, m.filter = false, ""
	case "tab":
		m.inTodos = !m.inTodos
	case "up", "k":
		if m.inTodos {
			m.todoSel--
		} else {
			m.sel, m.todoSel = m.sel-1, 0
		}
	case "down", "j":
		if m.inTodos {
			m.todoSel++
		} else {
			m.sel, m.todoSel = m.sel+1, 0
		}
	case " ":
		if m.inTodos {
			m.toggle()
		}
	case "/":
		m.filtering, m.inTodos = true, false
	case "r":
		return m, m.load()
	case "s":
		return m, m.run("sync")
	case "c":
		return m, m.run("clean")
	case "d":
		if r, ok := m.current(); ok {
			return m, m.assess(r)
		}
	}
	return m.clamp(), nil
}

func (m *model) toggle() {
	path, content, todos := m.todos()
	if m.todoSel >= len(todos) {
		return
	}
	t := todos[m.todoSel]
	out, err := ctx.SetDone(content, t.Index, !t.Done)
	if err == nil {
		err = ctx.Write(path, out)
	}
	if err != nil {
		m.status, m.statusErr = err.Error(), true
	}
}

func (m model) assess(r wt.Row) tea.Cmd {
	repo, cfg := m.repo, m.cfg
	return func() tea.Msg {
		c := confirm{folder: r.Folder}
		// Same refusals as git wt rm, so the modal never offers a delete rm would reject.
		if wts, err := repo.Worktrees(); err == nil {
			if main, _, err := wtsync.MainWorktree(repo, cfg, wts); err == nil && main != "" && wt.SamePath(main, r.WT.Path) {
				c.refuse = fmt.Sprintf("Refusing to remove the main worktree '%s'.", r.Folder)
			}
		}
		if c.refuse == "" && r.WT.Branch != "" && r.WT.Branch == cfg.BaseBranch {
			c.refuse = fmt.Sprintf("Refusing to delete the base branch '%s'.", r.WT.Branch)
		}
		if c.refuse != "" {
			return assessedMsg{c, nil}
		}
		merged, err := repo.MergedBranches(cfg.BaseBranch)
		if err != nil {
			merged = map[string]bool{}
		}
		var heads map[string]map[string]bool
		if r.WT.Branch != "" {
			heads = wt.RemoteHeads(repo, r.WT.Branch)
		}
		c.safety, err = wt.Assess(repo, r.WT, merged, heads)
		return assessedMsg{c, err}
	}
}

// --- view ---

var (
	fg     = func(c string) lipgloss.Style { return lipgloss.NewStyle().Foreground(lipgloss.Color(c)) }
	red    = fg("1")
	green  = fg("2")
	yellow = fg("11")
	blue   = fg("4")
	dim    = fg("8")
	accent = fg("5")
	bold   = lipgloss.NewStyle().Bold(true)
)

// fit truncates s to w cells with … and pads it to exactly w.
func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	s = ansi.Truncate(s, w, "…")
	return s + strings.Repeat(" ", w-lipgloss.Width(s))
}

// box draws a rounded border of exactly w×h with title in the top edge.
func box(title string, lines []string, w, h int, active bool) string {
	in := w - 2
	bc := dim
	if active {
		bc = accent
	}
	title = ansi.Truncate(" "+title+" ", max(in-1, 0), "…")
	out := []string{bc.Render("╭─") + bold.Render(title) + bc.Render(strings.Repeat("─", max(in-1-lipgloss.Width(title), 0))+"╮")}
	for i := 0; i < h-2; i++ {
		l := ""
		if i < len(lines) {
			l = lines[i]
		}
		out = append(out, bc.Render("│")+fit(l, in)+bc.Render("│"))
	}
	return strings.Join(append(out, bc.Render("╰"+strings.Repeat("─", in)+"╯")), "\n")
}

func remote(r wt.Row) string {
	switch {
	case r.Err != nil || r.WT.Branch == "" || r.WT.Detached:
		return "N/A"
	case !r.HasUpstream:
		return yellow.Render("no upstream")
	case r.Ahead > 0 && r.Behind > 0:
		return yellow.Render(fmt.Sprintf("↑%d ↓%d", r.Ahead, r.Behind))
	case r.Ahead > 0:
		return blue.Render(fmt.Sprintf("↑%d", r.Ahead))
	case r.Behind > 0:
		return red.Render(fmt.Sprintf("↓%d", r.Behind))
	}
	return green.Render("synced")
}

func changes(r wt.Row) string {
	switch {
	case r.Err != nil:
		return red.Render("error")
	case len(r.Changed) > 0:
		return red.Render(fmt.Sprintf("● %d", len(r.Changed)))
	}
	return green.Render("clean")
}

func (m model) View() string {
	switch {
	case m.w == 0:
		return ""
	case m.w < minW || m.h < minH:
		return tooSmall
	}
	bodyH := m.h - 2
	lw := m.w * 40 / 100
	rw := m.w - lw
	var body string
	switch {
	case m.confirm != nil:
		body = lipgloss.Place(m.w, bodyH, lipgloss.Center, lipgloss.Center, m.modal())
	default:
		left, right := m.listLines(lw-2, bodyH-2), m.detailLines(rw-2, bodyH-2)
		title := "Details"
		if r, ok := m.current(); ok {
			title = r.Folder
		}
		body = lipgloss.JoinHorizontal(lipgloss.Top, box("Worktrees", left, lw, bodyH, !m.inTodos), box(title, right, rw, bodyH, m.inTodos))
	}
	return m.header() + "\n" + body + "\n" + m.footer()
}

func (m model) header() string {
	l := bold.Render(" git wt · " + filepath.Base(m.repo.Root))
	if !m.loaded {
		return fit(l, m.w)
	}
	var dirty, merged int
	for _, r := range m.rows {
		if len(r.Changed) > 0 {
			dirty++
		}
		if r.Merged {
			merged++
		}
	}
	rt := fmt.Sprintf("%d worktrees · %d dirty · %d merged ", len(m.rows), dirty, merged)
	gap := m.w - lipgloss.Width(l) - lipgloss.Width(rt)
	if gap < 1 {
		return fit(l, m.w)
	}
	return l + strings.Repeat(" ", gap) + rt
}

func (m model) footer() string {
	switch {
	case m.filtering:
		return fit(" filter: "+m.filter+"▌  (enter apply · esc clear)", m.w)
	case m.status != "":
		st := dim
		if m.statusErr {
			st = red
		}
		return fit(" "+st.Render(m.status), m.w)
	}
	items := append([]string{}, helpItems...)
	if m.filter != "" {
		items = append([]string{"filter: " + m.filter + " (esc clears)"}, items...)
	}
	// Drop the least needed hints first so the action keys survive on narrow terminals.
	for _, drop := range []string{"↑↓ move", "space toggle", "tab todos", "r refresh", "/ filter"} {
		if lipgloss.Width(" "+strings.Join(items, " · ")) <= m.w {
			break
		}
		items = slices.DeleteFunc(items, func(s string) bool { return s == drop })
	}
	return fit(dim.Render(" "+strings.Join(items, " · ")), m.w)
}

func (m model) listLines(w, h int) []string {
	if !m.loaded {
		return []string{" Loading…"}
	}
	if m.err != nil {
		return []string{red.Render(" Error: " + m.err.Error())}
	}
	rows := m.visible()
	if len(rows) == 0 {
		return []string{dim.Render(" No matching worktrees")}
	}
	nameW := 0
	for _, r := range rows {
		nameW = max(nameW, lipgloss.Width(r.Folder))
	}
	nameW = min(nameW, max(w-2-8, 6)) // folder beats the remote column when narrow
	// Scroll so the selection stays on screen.
	start := 0
	if m.sel >= h {
		start = m.sel - h + 1
	}
	var out []string
	for i := start; i < len(rows) && i < start+h; i++ {
		r := rows[i]
		mark, name := "  ", fit(r.Folder, nameW)
		if i == m.sel {
			mark, name = accent.Render("▸ "), bold.Render(name)
		}
		mg := ""
		if r.Merged {
			mg = yellow.Render("merged")
		}
		out = append(out, mark+name+" "+fit(changes(r), 7)+" "+fit(remote(r), 11)+" "+mg)
	}
	return out
}

func (m model) detailLines(w, h int) []string {
	r, ok := m.current()
	if !ok {
		return nil
	}
	branch := r.WT.Branch
	if branch == "" || r.WT.Detached {
		branch = "(detached)"
	}
	path := r.WT.Path
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(path, home+string(os.PathSeparator)) {
		path = "~" + path[len(home):]
	}
	label := func(k, v string) string { return " " + dim.Render(fmt.Sprintf("%-8s ", k)) + v }
	out := []string{label("Branch", branch), label("Path", path), label("Remote", remote(r))}
	if r.Merged {
		out = append(out, label("Merged", yellow.Render("yes")))
	}
	if r.Err != nil {
		out = append(out, red.Render(" Error: "+r.Err.Error()))
	}
	_, _, todos := m.todos()
	out = append(out, "", bold.Render(" Changed files"))
	// Leave room for the todos block (heading + at least a few items) below the file list.
	room := max(h-len(out)-3-max(len(todos), 1), 1)
	switch {
	case len(r.Changed) == 0:
		out = append(out, dim.Render("  none"))
	case len(r.Changed) > room:
		for _, f := range r.Changed[:room-1] {
			out = append(out, "  "+f)
		}
		out = append(out, dim.Render(fmt.Sprintf("  … and %d more", len(r.Changed)-room+1)))
	default:
		for _, f := range r.Changed {
			out = append(out, "  "+f)
		}
	}
	out = append(out, "", bold.Render(" Context todos")+dim.Render("  (dx ctx)"))
	if len(todos) == 0 {
		out = append(out, dim.Render("  No todos"))
	}
	for i, t := range todos {
		mark, box := "  ", "[ ]"
		if t.Done {
			box = green.Render("[✓]")
		}
		if m.inTodos && i == m.todoSel {
			mark = accent.Render("▸ ")
		}
		out = append(out, mark+box+" "+t.Text)
	}
	return out
}

func (c confirm) canDelete() bool { return c.refuse == "" && c.safety.Safe() }

func (m model) modal() string {
	c := m.confirm
	s := c.safety
	lost := s.LostCommits()
	var tail []string
	switch {
	case c.refuse != "":
		tail = append(tail, red.Render(c.refuse))
	case len(s.Uncommitted) == 0 && lost == 0:
		tail = append(tail, green.Render("nothing would be lost"))
	}
	if lost > 0 {
		tail = append(tail, red.Render(fmt.Sprintf("%d unpushed commit(s)", lost)))
	}
	if c.refuse == "" && !s.Safe() {
		tail = append(tail, "", yellow.Render("run git wt rm -f "+c.folder+" in a shell to override"))
	}
	if c.canDelete() {
		tail = append(tail, "", "y confirm · n cancel")
	} else {
		tail = append(tail, "", "n/esc close")
	}
	lines := []string{bold.Render("Delete worktree " + c.folder + " and its branch?"), ""}
	if n := len(s.Uncommitted); n > 0 {
		// Cap the list so the prompt always stays on screen.
		shown := min(n, maxLost, max(m.h-4-len(lines)-len(tail)-2, 0))
		lines = append(lines, red.Render(fmt.Sprintf("%d uncommitted file(s):", n)))
		for _, f := range s.Uncommitted[:shown] {
			lines = append(lines, "  "+f)
		}
		if n > shown {
			lines = append(lines, dim.Render(fmt.Sprintf("  and %d more", n-shown)))
		}
	}
	lines = append(lines, tail...)
	for i := range lines {
		lines[i] = " " + lines[i]
	}
	return box("Confirm delete", lines, min(m.w-4, 72), min(len(lines)+2, m.h-2), true)
}
