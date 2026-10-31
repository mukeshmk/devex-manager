// Package wtapp is the full-screen `git wt` app: lazygit-style panels, a main view, popups and single-key actions.
package wtapp

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/mukeshmk/devex-manager/internal/ctx"
	"github.com/mukeshmk/devex-manager/internal/gitwt/sync"
	"github.com/mukeshmk/devex-manager/internal/gitwt/worktree"
	"github.com/mukeshmk/devex-manager/internal/shared/config"
	"github.com/mukeshmk/devex-manager/internal/shared/gitx"
	"github.com/mukeshmk/devex-manager/internal/shared/ui"
)

const (
	minW, minH = 60, 12
	tooSmall   = "Terminal too small (need 60×12)"
	maxLost    = 10
)

// keyHelp is one key: the help popup lists every entry, the key-hint line shows those with a hint.
type keyHelp struct {
	keys, desc, hint string
	on               panel // 0: every panel
}

var helpItems = []keyHelp{
	{"1-5", "focus a panel", "1-5 panels", 0},
	{"tab →/l", "next panel", "", 0},
	{"shift+tab ←/h", "previous panel", "", 0},
	{"↑/↓ j/k", "move, or scroll the main view or log", "↑↓ move", 0},
	{"enter", "focus the main view so it scrolls", "", 0},
	{"esc", "leave the main view, or clear the filter", "", 0},
	{"/", "filter worktrees", "/ filter", panelWorktrees},
	{"r", "refresh", "", 0},
	{"s", "sync copy paths", "s sync", 0},
	{"c", "clean merged worktrees", "c clean", 0},
	{"d", "delete the selected worktree", "d delete", 0},
	{"a", "add a todo", "a add todo", panelContext},
	{"space", "toggle the selected todo", "space toggle", panelContext},
	{"x", "delete the selected todo", "x delete todo", panelContext},
	{"C", "clear done todos", "C clear done", panelContext},
	{"q ctrl+c", "quit", "q quit", 0},
	{"?", "this help", "? keys", 0},
}

type (
	loadedMsg struct {
		rows            []wt.Row
		err             error
		syncLines, plan []string
		syncErr         error
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
	path   string // what rm is given: a folder name can also be some other worktree's branch
	safety wt.Safety
	refuse string // non-empty: git wt rm would refuse this outright, so the modal only offers close
}

type model struct {
	repo *gitx.Repo
	cfg  config.RepoConfig
	self string
	exec func(*exec.Cmd) tea.Cmd // injectable so tests can see argv without running anything

	w, h              int
	rows              []wt.Row
	loaded            bool
	err               error
	syncLines, plan   []string // sync --check lines and plan summaries; empty with no copy paths
	syncErr           error
	sel, todoSel      int
	scroll, logScroll int // main view offset from the top, log offset from its tail
	filter            string
	filtering         bool
	focus             panel
	focusMain         bool
	confirm           *confirm
	popup             *popup
	log               logRing
	busy              string // name of the running flow
	statusMsg         string
	statusErr         bool
}

// New builds the app; self is the path of the running binary, re-invoked as git-wt for actions.
func New(repo *gitx.Repo, cfg config.RepoConfig, self string) tea.Model {
	return model{repo: repo, cfg: cfg, self: self, focus: panelWorktrees, exec: func(c *exec.Cmd) tea.Cmd {
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
		msg := loadedMsg{}
		msg.rows, msg.err = wt.Collect(repo, cfg)
		if len(cfg.CopyPaths) > 0 {
			s, err := wtsync.Session(repo, cfg)
			if err == nil {
				_, msg.syncLines = s.InSync()
				for _, st := range s.Plan().Steps {
					msg.plan = append(msg.plan, "  "+st.Summary)
				}
			}
			msg.syncErr = err
		}
		return msg
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
		m.syncLines, m.plan, m.syncErr = msg.syncLines, msg.plan, msg.syncErr
		if msg.syncErr != nil {
			m.statusMsg, m.statusErr = msg.syncErr.Error(), true
		}
		m = m.clamp()
	case assessedMsg:
		if msg.err != nil {
			m.statusMsg, m.statusErr = "Error: "+msg.err.Error(), true
		} else {
			m.confirm = &msg.confirm
		}
	case execDoneMsg:
		// The child's output vanishes with the alt screen, so say how it ended.
		m.statusMsg, m.statusErr = strings.Join(msg.args, " ")+": done", msg.err != nil
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("%s: %v", strings.Join(msg.args, " "), msg.err)
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
	m.statusMsg = ""
	if m.popup != nil {
		p, done, _, cmd := m.popup.update(msg)
		m.popup = &p
		if done {
			m.popup = nil
		}
		return m, cmd
	}
	if m.confirm != nil {
		switch k {
		case "y":
			if !m.confirm.canDelete() {
				return m, nil
			}
			p := m.confirm.path
			m.confirm = nil
			return m, m.run("rm", p)
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
	case "?":
		m.popup = &popup{kind: popupHelp, title: "Keys", options: helpLines()}
	case "1", "2", "3", "4", "5":
		m.focus, m.focusMain, m.logScroll = panel(k[0]-'0'), false, 0
	case "tab", "right", "l":
		m.focus, m.focusMain, m.logScroll = m.focus%5+1, false, 0
	case "shift+tab", "left", "h":
		m.focus, m.focusMain, m.logScroll = (m.focus+3)%5+1, false, 0
	case "enter":
		if m.focus != panelLog {
			m.focusMain, m.scroll = true, 0
		}
	case "esc":
		if m.focusMain {
			m.focusMain = false
		} else {
			m.filter = ""
		}
	case "up", "k":
		m = m.move(-1)
	case "down", "j":
		m = m.move(1)
	case " ":
		if m.focus == panelContext && !m.focusMain {
			m.toggle()
		}
	case "/":
		m.filtering, m.focus, m.focusMain = true, panelWorktrees, false
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

// move handles ↑/↓ for whatever has focus.
func (m model) move(d int) model {
	_, mr, lr, _ := layout(m.w, m.h)
	switch {
	case m.focusMain:
		_, lines := m.mainView()
		m.scroll = min(max(m.scroll+d, 0), max(len(lines)-(mr.h-2), 0))
	case m.focus == panelLog:
		m.logScroll = min(max(m.logScroll-d, 0), max(len(m.log.Lines())-(lr.h-2), 0))
	case m.focus == panelContext:
		m.todoSel += d
	case m.focus == panelWorktrees:
		m.sel, m.todoSel = m.sel+d, 0
	}
	return m
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
		m.statusMsg, m.statusErr = err.Error(), true
	}
}

func (m model) assess(r wt.Row) tea.Cmd {
	repo, cfg := m.repo, m.cfg
	return func() tea.Msg {
		c := confirm{folder: r.Folder, path: r.WT.Path}
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
		c.safety, err = wt.Assess(repo, cfg, r.WT, merged, heads)
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

// edge is the box border colour: accent marks the focused pane.
func edge(active bool) lipgloss.Style {
	if active {
		return accent
	}
	return dim
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

func (c confirm) canDelete() bool { return c.refuse == "" && c.safety.Safe() }

func (m model) modal() string {
	c := m.confirm
	s := c.safety
	lost := s.LostCommits()
	var tail []string
	switch {
	case c.refuse != "":
		tail = append(tail, red.Render(c.refuse))
	case len(s.Uncommitted) == 0 && lost == 0 && s.GoneCommits() == 0:
		tail = append(tail, green.Render("nothing would be lost"))
	}
	if lost > 0 {
		tail = append(tail, red.Render(fmt.Sprintf("%d unpushed commit(s)", lost)))
	}
	if n := s.GoneCommits(); n > 0 {
		tail = append(tail, yellow.Render(fmt.Sprintf("%d local commit(s) not on any remote (upstream gone)", n)))
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
	return ui.Box("Confirm delete", lines, min(m.w-4, 72), min(len(lines)+2, m.h-2), accent)
}
