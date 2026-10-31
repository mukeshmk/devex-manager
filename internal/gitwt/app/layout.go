package wtapp

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/mukeshmk/devex-manager/internal/shared/ui"
)

type panel int

const (
	panelStatus panel = iota + 1
	panelWorktrees
	panelContext
	panelSync
	panelLog
)

var panelTitles = [...]string{panelStatus: "Status", panelWorktrees: "Worktrees", panelContext: "Context", panelSync: "Sync", panelLog: "Command log"}

// focusMark follows the focused panel's title, so focus shows without colour too.
const focusMark = " ◆"

type rect struct{ x, y, w, h int }

// layout splits the rows above the key-hint line. left is Status, Worktrees, Context, Sync in that order.
func layout(w, h int) (left []rect, main, log rect, stacked bool) {
	bodyH := h - 1
	if w >= 80 {
		lw, rest := w/3, bodyH-4
		ch := max(rest/4, 2)
		wh := rest - 2*ch
		left = []rect{{0, 0, lw, 4}, {0, 4, lw, wh}, {0, 4 + wh, lw, ch}, {0, 4 + wh + ch, lw, rest - wh - ch}}
		mh := bodyH * 70 / 100
		return left, rect{lw, 0, w - lw, mh}, rect{lw, mh, w - lw, bodyH - mh}, false
	}
	// Narrow: the four panels as a 2×2 grid (Status|Sync over Worktrees|Context) so everything fits in 12 rows.
	hw, rest := w/2, bodyH-4-3
	mh := max(rest*45/100, 2)
	gh := rest - mh
	left = []rect{{0, 0, hw, 4}, {0, 4, hw, gh}, {hw, 4, w - hw, gh}, {hw, 0, w - hw, 4}}
	return left, rect{0, 4 + gh, w, mh}, rect{0, 4 + gh + mh, w, 3}, true
}

type placed struct {
	r   rect
	box string // exactly r.w × r.h
}

// compose pastes tiling boxes into h rows, left to right.
func compose(h int, ps []placed) string {
	slices.SortStableFunc(ps, func(a, b placed) int { return cmp.Compare(a.r.x, b.r.x) })
	rows := make([]string, h)
	for _, p := range ps {
		for j, l := range strings.Split(p.box, "\n") {
			if y := p.r.y + j; y < h {
				rows[y] += l
			}
		}
	}
	return strings.Join(rows, "\n")
}

func (m model) View() string {
	switch {
	case m.w == 0:
		return ""
	case m.w < minW || m.h < minH:
		return tooSmall
	}
	bodyH := m.h - 1
	var body string
	switch {
	case m.confirm != nil:
		body = lipgloss.Place(m.w, bodyH, lipgloss.Center, lipgloss.Center, m.modal())
	case m.popup != nil:
		body = lipgloss.Place(m.w, bodyH, lipgloss.Center, lipgloss.Center, m.popup.view(m.w, bodyH))
	default:
		body = m.panels()
	}
	return body + "\n" + m.footer()
}

func (m model) panels() string {
	left, mr, lr, _ := layout(m.w, m.h)
	var ps []placed
	for i, r := range left {
		p := panel(i + 1)
		ps = append(ps, placed{r, m.box(p, m.panelLines(p, r.w-2, r.h-2), r)})
	}
	title, lines := m.mainView()
	// Scroll offsets only apply while focused; enter resets scroll, and the log falls back to its tail.
	if m.focusMain {
		lines = lines[min(m.scroll, max(len(lines)-(mr.h-2), 0)):]
		title += focusMark
	}
	ps = append(ps, placed{mr, ui.Box(title, lines, mr.w, mr.h, edge(m.focusMain))})
	ps = append(ps, placed{lr, m.box(panelLog, m.logLines(lr.h-2), lr)})
	return compose(m.h-1, ps)
}

func (m model) box(p panel, lines []string, r rect) string {
	title := fmt.Sprintf("[%d] %s", p, panelTitles[p])
	on := m.focus == p && !m.focusMain
	if on {
		title += focusMark
	}
	return ui.Box(title, lines, r.w, r.h, edge(on))
}

// clean makes free text safe to measure: tabs and carriage returns would move the cursor unseen.
var clean = strings.NewReplacer("\t", "    ", "\r", "").Replace

func (m model) panelLines(p panel, w, h int) []string {
	switch p {
	case panelStatus:
		out := []string{" " + filepath.Base(m.repo.Root) + " · base " + m.cfg.BaseBranch}
		if m.loaded {
			out = append(out, " "+m.syncSummary())
		}
		return out
	case panelWorktrees:
		return m.listLines(w, h)
	case panelContext:
		_, _, todos := m.todos()
		if len(todos) == 0 {
			if r, ok := m.current(); ok && r.WT.Detached {
				return []string{dim.Render(" No todos (detached HEAD)")}
			}
			return []string{dim.Render(" No todos")}
		}
		var out []string
		for i, t := range todos {
			mark, box := "  ", "[ ]"
			if t.Done {
				box = green.Render("[✓]")
			}
			if m.focus == panelContext && i == m.todoSel {
				mark = accent.Render("▸ ")
			}
			out = append(out, mark+box+" "+clean(t.Text))
		}
		return out[max(m.todoSel-h+1, 0):]
	case panelSync:
		var out []string
		for _, l := range m.syncLines {
			if !strings.HasPrefix(l, " ") {
				out = append(out, " "+yellow.Render(l))
			}
		}
		return out
	}
	return nil
}

func (m model) syncSummary() string {
	n := 0
	for _, l := range m.syncLines {
		if !strings.HasPrefix(l, " ") {
			n++
		}
	}
	switch {
	case len(m.cfg.CopyPaths) == 0:
		return dim.Render("sync: no copy paths")
	case m.syncErr != nil:
		return red.Render("sync: error")
	case n > 0:
		return yellow.Render(fmt.Sprintf("sync: %d files out of sync", n))
	}
	return green.Render("sync: in sync")
}

// mainView is the right-hand view for the focused panel; the log panel keeps the worktree details.
func (m model) mainView() (string, []string) {
	switch m.focus {
	case panelStatus:
		var dirty, merged int
		for _, r := range m.rows {
			if len(r.Changed) > 0 {
				dirty++
			}
			if r.Merged {
				merged++
			}
		}
		out := []string{" " + bold.Render("git wt · "+filepath.Base(m.repo.Root)), " " + m.repo.Root,
			fmt.Sprintf(" base %s · %d worktrees · %d dirty · %d merged", m.cfg.BaseBranch, len(m.rows), dirty, merged), "", " " + m.syncSummary()}
		for _, l := range m.syncLines {
			out = append(out, " "+l)
		}
		return "Status", out
	case panelContext:
		path, content, _ := m.todos()
		if path == "" {
			return "Context", []string{dim.Render(" No context file for this branch (dx ctx)")}
		}
		var out []string
		for _, l := range strings.Split(strings.TrimRight(content, "\n"), "\n") {
			out = append(out, " "+clean(l))
		}
		return "Context · " + filepath.Base(path), out
	case panelSync:
		switch {
		case len(m.cfg.CopyPaths) == 0:
			return "Sync plan", []string{dim.Render(" No copy paths configured in .devex.conf (copies.paths).")}
		case len(m.plan) == 0 && m.syncErr == nil:
			return "Sync plan", []string{green.Render(" ✓ Everything is in sync.")}
		}
		return "Sync plan", m.plan
	}
	title := "Details"
	if r, ok := m.current(); ok {
		title = r.Folder
	}
	return title, m.detailLines()
}

func (m model) logLines(h int) []string {
	l := m.log.Lines()
	if len(l) == 0 {
		return []string{dim.Render(" (no commands yet)")}
	}
	end := len(l)
	if m.focus == panelLog {
		end = max(end-m.logScroll, 0)
	}
	var out []string
	for _, s := range l[max(end-h, 0):end] {
		out = append(out, " "+clean(s))
	}
	return out
}

func (m model) footer() string {
	switch {
	case m.filtering:
		return ui.Fit(" filter: "+m.filter+"▌  (enter apply · esc clear)", m.w)
	case m.statusMsg != "": // before busy, so "wait for sync to finish" shows
		st := dim
		if m.statusErr {
			st = red
		}
		return ui.Fit(" "+st.Render(m.statusMsg), m.w)
	case m.busy != "":
		return ui.Fit(" "+yellow.Render(m.busy+" running…"), m.w)
	}
	var own, all []string
	for _, k := range helpItems {
		switch {
		case k.hint == "":
		case k.on == m.focus:
			own = append(own, k.hint)
		case k.on == 0:
			all = append(all, k.hint)
		}
	}
	items := append(own, all...)
	if m.filter != "" {
		items = append([]string{"filter: " + m.filter + " (esc clears)"}, items...)
	}
	// Drop global hints from the end, keeping the panel's own keys and the final "? keys".
	for len(items) > 1 && lipgloss.Width(" "+strings.Join(items, " · ")) > m.w {
		items = slices.Delete(items, len(items)-2, len(items)-1)
	}
	return ui.Fit(dim.Render(" "+strings.Join(items, " · ")), m.w)
}

func helpLines() []string {
	var out []string
	for _, k := range helpItems {
		l := " " + ui.Fit(k.keys, 14) + " " + k.desc
		if k.on != 0 {
			l += dim.Render(" (" + panelTitles[k.on] + ")")
		}
		out = append(out, l)
	}
	return out
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
		mark, name := "  ", ui.Fit(r.Folder, nameW)
		if i == m.sel {
			mark, name = accent.Render("▸ "), bold.Render(name)
		}
		mg := ""
		if r.Merged {
			mg = yellow.Render("merged")
		}
		out = append(out, mark+name+" "+ui.Fit(changes(r), 7)+" "+ui.Fit(remote(r), 11)+" "+mg)
	}
	return out
}

func (m model) detailLines() []string {
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
	out = append(out, "", bold.Render(" Changed files"))
	if len(r.Changed) == 0 {
		out = append(out, dim.Render("  none"))
	}
	for _, f := range r.Changed {
		out = append(out, "  "+clean(f))
	}
	return out
}
