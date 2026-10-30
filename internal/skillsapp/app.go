// Package skillsapp is the full-screen `dx skills` grid: skills × locations, diff pane, tick-and-apply.
package skillsapp

import (
	"fmt"
	"slices"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/mukeshmk/devex-manager/internal/config"
	"github.com/mukeshmk/devex-manager/internal/skills"
)

const (
	minW, minH = 60, 12
	tooSmall   = "Terminal too small (need 60×12)"
)

var (
	helpItems = []string{"↑↓ move", "space tick", "a apply", "pgup/pgdn diff", "/ filter", "q quit"}
	bold      = lipgloss.NewStyle().Bold(true)
	dim       = lipgloss.NewStyle().Faint(true)
	accent    = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	red       = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	green     = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	yellow    = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	blue      = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
	markStyle = map[rune]lipgloss.Style{'★': yellow, '≠': red, '✓': green, '·': dim}
)

type (
	loadedMsg struct {
		skills []skills.Skill
		marks  map[string][]rune // per skill, one mark per location in m.locs order
		ups    []skills.Update
		err    error
	}
	appliedMsg struct {
		root string
		done int
		err  error
	}
)

type model struct {
	cfg  config.SkillsConfig
	locs []string // master, then each tool

	w, h      int
	loaded    bool
	err       error
	skills    []skills.Skill
	marks     map[string][]rune
	ups       []skills.Update
	unticked  map[string]bool // zero value = ticked, so new pending updates start ticked
	diffs     map[string]string
	sel       int
	scroll    int
	filter    string
	filtering bool
	status    string
}

// New builds the grid for cfg; skills are scanned asynchronously from Init.
func New(cfg config.SkillsConfig) tea.Model {
	locs := []string{"master"}
	for _, t := range cfg.Tools {
		locs = append(locs, t.Name)
	}
	return model{cfg: cfg, locs: locs, unticked: map[string]bool{}, diffs: map[string]string{}}
}

// Run shows m full-screen until the user quits.
func Run(m tea.Model) error {
	_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

func (m model) Init() tea.Cmd { return m.load() }

func (m model) load() tea.Cmd {
	cfg, locs := m.cfg, m.locs
	return func() tea.Msg {
		sk, err := skills.Scan(cfg)
		if err != nil {
			return loadedMsg{err: err}
		}
		// Marks and Plan share one cached comparison, so each pair is diffed once per load.
		cache := map[[2]string]bool{}
		same := func(a, b string) bool {
			k := [2]string{a, b}
			if v, ok := cache[k]; ok {
				return v
			}
			cache[k] = skills.SameDir(a, b)
			return cache[k]
		}
		marks := map[string][]rune{}
		for _, s := range sk {
			newest := dirOf(s, s.Newest)
			for _, l := range locs {
				d := dirOf(s, l)
				switch {
				case d == "":
					marks[s.Name] = append(marks[s.Name], '·')
				case l == s.Newest:
					marks[s.Name] = append(marks[s.Name], '★')
				case same(newest, d):
					marks[s.Name] = append(marks[s.Name], '✓')
				default:
					marks[s.Name] = append(marks[s.Name], '≠')
				}
			}
		}
		return loadedMsg{skills: sk, marks: marks, ups: skills.Plan(cfg, sk, same)}
	}
}

func dirOf(s skills.Skill, loc string) string {
	for _, c := range s.Copies {
		if c.Location == loc {
			return c.Dir
		}
	}
	return ""
}

func (m model) visible() []skills.Skill {
	if m.filter == "" {
		return m.skills
	}
	var out []skills.Skill
	for _, s := range m.skills {
		if strings.Contains(strings.ToLower(s.Name), strings.ToLower(m.filter)) {
			out = append(out, s)
		}
	}
	return out
}

func (m model) current() (skills.Skill, bool) {
	v := m.visible()
	if m.sel < len(v) {
		return v[m.sel], true
	}
	return skills.Skill{}, false
}

func (m model) pending(name string) []skills.Update {
	var out []skills.Update
	for _, u := range m.ups {
		if u.Skill == name {
			out = append(out, u)
		}
	}
	return out
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
	case loadedMsg:
		m.loaded, m.err, m.skills, m.marks, m.ups = true, msg.err, msg.skills, msg.marks, msg.ups
		m.diffs = map[string]string{}
		m.sel = min(m.sel, max(len(m.visible())-1, 0))
	case appliedMsg:
		m.status = green.Render(fmt.Sprintf("Applied %d update(s)", msg.done))
		if msg.err != nil {
			m.status = red.Render(fmt.Sprintf("%s (%d applied)", msg.err, msg.done))
		}
		if msg.root != "" {
			m.status += " · Backups stored in: " + msg.root
		}
		m.loaded = false
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
			m.filter += string(msg.Runes)
		}
		m.sel, m.scroll = 0, 0
		return m, nil
	}
	m.status = ""
	page := max((m.h-4)/2, 1)
	switch k {
	case "q":
		return m, tea.Quit
	case "up", "k":
		if m.sel > 0 {
			m.sel, m.scroll = m.sel-1, 0
		}
	case "down", "j":
		if m.sel < len(m.visible())-1 {
			m.sel, m.scroll = m.sel+1, 0
		}
	case "pgdown", "J":
		m.scroll += page
		if s, ok := m.current(); ok {
			m.scroll = min(m.scroll, strings.Count(m.diffs[s.Name], "\n"))
		}
	case "pgup", "K":
		m.scroll = max(m.scroll-page, 0)
	case "/":
		m.filtering = true
	case "esc":
		m.filter, m.sel, m.scroll = "", 0, 0
	case " ", "space":
		if s, ok := m.current(); ok && len(m.pending(s.Name)) > 0 {
			m.unticked[s.Name] = !m.unticked[s.Name]
		}
	case "a":
		if !m.loaded {
			return m, nil
		}
		var ups []skills.Update
		for _, u := range m.ups {
			if !m.unticked[u.Skill] {
				ups = append(ups, u)
			}
		}
		if len(ups) == 0 {
			m.status = "Nothing ticked to apply"
			return m, nil
		}
		master := m.cfg.MasterDir
		m.loaded = false // blocks a second apply until the rescan lands
		return m, func() tea.Msg {
			root, done, err := skills.ApplyUpdates(ups, master, time.Now())
			return appliedMsg{root, done, err}
		}
	}
	return m, nil
}

// fit truncates s to w cells with … and pads it to exactly w.
func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	s = ansi.Truncate(s, w, "…")
	return s + strings.Repeat(" ", w-lipgloss.Width(s))
}

// box draws a rounded border of exactly w×h with title in the top edge.
func box(title string, lines []string, w, h int) string {
	in := w - 2
	title = ansi.Truncate(" "+title+" ", max(in-1, 0), "…")
	out := []string{dim.Render("╭─") + bold.Render(title) + dim.Render(strings.Repeat("─", max(in-1-lipgloss.Width(title), 0))+"╮")}
	for i := 0; i < h-2; i++ {
		l := ""
		if i < len(lines) {
			l = lines[i]
		}
		out = append(out, dim.Render("│")+fit(l, in)+dim.Render("│"))
	}
	return strings.Join(append(out, dim.Render("╰"+strings.Repeat("─", in)+"╯")), "\n")
}

func (m model) View() string {
	switch {
	case m.w == 0:
		return ""
	case m.w < minW || m.h < minH:
		return tooSmall
	}
	bodyH := m.h - 2
	if m.status != "" {
		bodyH--
	}
	nameW := 5
	for _, s := range m.skills {
		nameW = max(nameW, lipgloss.Width(s.Name))
	}
	need := 2 + 6 + min(nameW, 28) // border + "› [x] " + name
	for _, l := range m.locs {
		need += lipgloss.Width(l) + 1
	}
	lw := max(min(need, m.w*60/100), m.w*35/100)
	rw := m.w - lw
	title := "Diff"
	if s, ok := m.current(); ok {
		title = s.Name
	}
	body := lipgloss.JoinHorizontal(lipgloss.Top, box("Skills", m.gridLines(lw-2, bodyH-2), lw, bodyH),
		box(title, m.diffLines(bodyH-2), rw, bodyH))
	out := []string{m.header(), body}
	if m.status != "" {
		out = append(out, fit(" "+m.status, m.w))
	}
	return strings.Join(append(out, m.footer()), "\n")
}

func (m model) header() string {
	l := bold.Render(" dx skills · " + m.cfg.MasterDir)
	if !m.loaded {
		return fit(l, m.w)
	}
	rt := fmt.Sprintf("%d skills · %d pending ", len(m.skills), len(m.ups))
	gap := m.w - lipgloss.Width(l) - lipgloss.Width(rt)
	if gap < 1 {
		return fit(l, m.w)
	}
	return l + strings.Repeat(" ", gap) + rt
}

func (m model) footer() string {
	if m.filtering {
		return fit(" filter: "+m.filter+"▌  (enter apply · esc clear)", m.w)
	}
	items := append([]string{}, helpItems...)
	if m.filter != "" {
		items = append([]string{"filter: " + m.filter + " (esc clears)"}, items...)
	}
	// Drop the least needed hints first so the action keys survive on narrow terminals.
	for _, drop := range []string{"↑↓ move", "pgup/pgdn diff", "/ filter", "space tick"} {
		if lipgloss.Width(" "+strings.Join(items, " · ")) <= m.w {
			break
		}
		items = slices.DeleteFunc(items, func(s string) bool { return s == drop })
	}
	return fit(dim.Render(" "+strings.Join(items, " · ")), m.w)
}

func (m model) gridLines(w, h int) []string {
	if !m.loaded {
		return []string{" Loading…"}
	}
	if m.err != nil {
		return []string{red.Render(" " + m.err.Error())}
	}
	cols := 0
	for _, l := range m.locs {
		cols += lipgloss.Width(l) + 1
	}
	nameW := max(w-6-cols, 4)
	hdr := "      " + fit("Skill", nameW)
	for _, l := range m.locs {
		hdr += " " + l
	}
	out := []string{bold.Render(hdr)}
	v := m.visible()
	if len(v) == 0 {
		return append(out, dim.Render(" No skills"))
	}
	first := max(m.sel-(h-2), 0) // keep the selection inside the h-1 visible rows
	for i := first; i < len(v) && len(out) < h; i++ {
		s := v[i]
		cur, tick := "  ", "    "
		if i == m.sel {
			cur = accent.Render("› ")
		}
		if len(m.pending(s.Name)) > 0 {
			tick = "[x] "
			if m.unticked[s.Name] {
				tick = "[ ] "
			}
		}
		row := cur + tick + fit(s.Name, nameW)
		for j, l := range m.locs {
			lw := lipgloss.Width(l)
			mk := m.marks[s.Name][j]
			row += " " + strings.Repeat(" ", (lw-1)/2) + markStyle[mk].Render(string(mk)) + strings.Repeat(" ", lw-1-(lw-1)/2)
		}
		out = append(out, row)
	}
	return out
}

// diffLines is the newest copy against the first differing one, scrolled by m.scroll.
func (m model) diffLines(h int) []string {
	s, ok := m.current()
	if !m.loaded || !ok {
		return nil
	}
	d, ok := m.diffs[s.Name]
	if !ok {
		d = m.diff(s)
		m.diffs[s.Name] = d // map is shared across model copies: the cache survives View's value receiver
	}
	ls := strings.Split(strings.TrimRight(d, "\n"), "\n")
	start := min(m.scroll, max(len(ls)-h, 0))
	var out []string
	for _, l := range ls[start:] {
		l = strings.ReplaceAll(l, "\t", "    ")
		switch {
		case strings.HasPrefix(l, "+++"), strings.HasPrefix(l, "---"), strings.HasPrefix(l, "diff "):
			l = bold.Render(l)
		case strings.HasPrefix(l, "+"):
			l = green.Render(l)
		case strings.HasPrefix(l, "-"):
			l = red.Render(l)
		case strings.HasPrefix(l, "@@"):
			l = blue.Render(l)
		}
		out = append(out, " "+l)
	}
	return out
}

func (m model) diff(s skills.Skill) string {
	mk, newest := m.marks[s.Name], dirOf(s, s.Newest)
	for j, l := range m.locs {
		if mk[j] == '≠' {
			d, err := skills.Diff(newest, dirOf(s, l))
			if err != nil {
				return "Error: " + err.Error()
			}
			return fmt.Sprintf("%s (newest) vs %s\n\n%s", s.Newest, l, d)
		}
	}
	if len(s.Copies) == 1 {
		return "Only in " + s.Newest + "."
	}
	return "In sync across every copy."
}
