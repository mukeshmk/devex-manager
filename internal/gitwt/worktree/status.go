package wt

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/mukeshmk/devex-manager/internal/shared/config"
	"github.com/mukeshmk/devex-manager/internal/shared/gitx"
	"github.com/mukeshmk/devex-manager/internal/shared/ui"
)

// Row is the status of one worktree.
type Row struct {
	WT          gitx.Worktree
	Folder      string
	Changed     []string
	Ahead       int
	Behind      int
	HasUpstream bool
	Merged      bool
	Modified    time.Time
	Err         error
}

// Collect gathers a Row per non-bare worktree (Worktrees() order), one goroutine each.
func Collect(repo *gitx.Repo, cfg config.RepoConfig) ([]Row, error) {
	all, err := repo.Worktrees()
	if err != nil {
		return nil, err
	}
	merged, err := repo.MergedBranches(cfg.BaseBranch)
	if err != nil {
		merged = map[string]bool{} // base branch missing: nothing counts as merged
	}
	var wts []gitx.Worktree
	for _, w := range all {
		if !w.Bare {
			wts = append(wts, w)
		}
	}
	rows := make([]Row, len(wts))
	var wg sync.WaitGroup
	for i, w := range wts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := Row{WT: w, Folder: filepath.Base(w.Path)}
			r.Merged = w.Branch != "" && !w.Detached && w.Branch != cfg.BaseBranch && merged[w.Branch]
			if fi, err := os.Stat(w.Path); err != nil {
				r.Err = err
			} else {
				r.Modified = fi.ModTime()
			}
			if w.Prunable && r.Err == nil {
				r.Err = fmt.Errorf("worktree %s is prunable", w.Path)
			}
			if r.Err == nil {
				if r.Changed, r.Err = gitx.ChangedFiles(w.Path); r.Err == nil && w.Branch != "" && !w.Detached {
					// ahead/behind errors (e.g. gone upstream) leave the counts at zero, like Bash
					r.Ahead, r.Behind, r.HasUpstream, _ = gitx.AheadBehind(w.Path, w.Branch)
				}
			}
			rows[i] = r
		}()
	}
	wg.Wait()
	return rows, nil
}

func relTime(t, now time.Time) string {
	d := now.Sub(t)
	switch {
	case t.IsZero():
		return "Unknown"
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}

// RenderTable renders rows as a width-aware table followed by the summary.
func RenderTable(rows []Row, width int) string {
	st := func(c string) lipgloss.Style { return lipgloss.NewStyle().Foreground(lipgloss.Color(c)) }
	red, green, yellow, blue := st("1"), st("2"), st("11"), st("4")
	bold := lipgloss.NewStyle().Bold(true)
	cols := []ui.Column{{Title: "Worktree", Min: 8}, {Title: "Branch", Min: 10, Flex: true},
		{Title: "Changes", Min: 8}, {Title: "Remote", Min: 8}, {Title: "Merged", Min: 3}, {Title: "Last edited", Min: 8}}
	var cells [][]string
	var clean, dirty, synced, pending, merged int
	now := time.Now()
	for _, r := range rows {
		branch, changes, remote, mg := r.WT.Branch, "", "", green.Render("No")
		if branch == "" || r.WT.Detached {
			branch = "(detached)"
		}
		switch {
		case r.Err != nil:
			changes, remote = red.Render("Error"), "N/A"
		default:
			if len(r.Changed) > 0 {
				changes = red.Render(fmt.Sprintf("● %d files", len(r.Changed)))
				dirty++
			} else {
				changes = green.Render("○ Clean")
				clean++
			}
			switch {
			case branch == "(detached)":
				remote = "N/A"
			case !r.HasUpstream:
				remote = yellow.Render("No upstream")
			case r.Ahead > 0 && r.Behind > 0:
				remote, pending = yellow.Render(fmt.Sprintf("↑%d ↓%d", r.Ahead, r.Behind)), pending+1
			case r.Ahead > 0:
				remote, pending = blue.Render(fmt.Sprintf("↑%d", r.Ahead)), pending+1
			case r.Behind > 0:
				remote, pending = red.Render(fmt.Sprintf("↓%d", r.Behind)), pending+1
			default:
				remote, synced = green.Render("Synced"), synced+1
			}
		}
		if r.Merged {
			mg = yellow.Render("Yes")
			merged++
		}
		mod := "Unknown"
		if !r.Modified.IsZero() {
			mod = r.Modified.Format("2006-01-02 15:04")
			if width < 120 {
				mod = relTime(r.Modified, now)
			}
		}
		cells = append(cells, []string{r.Folder, branch, changes, remote, mg, mod})
	}
	var b strings.Builder
	b.WriteString(ui.Table(cols, cells, width) + "\n")
	fmt.Fprintf(&b, "%s\n  Total Worktrees: %s\n", bold.Render("Summary:"), bold.Render(fmt.Sprint(len(rows))))
	fmt.Fprintf(&b, "  Changes:         %s, %s\n", green.Render(fmt.Sprintf("%d Clean", clean)), red.Render(fmt.Sprintf("%d Dirty", dirty)))
	fmt.Fprintf(&b, "  Sync:            %s, %s\n", green.Render(fmt.Sprintf("%d Synced", synced)), yellow.Render(fmt.Sprintf("%d Pending", pending)))
	if merged > 0 {
		fmt.Fprintf(&b, "  Stale:           %s (clean up with %s)\n", yellow.Render(fmt.Sprintf("%d Merged", merged)), bold.Render("git wt clean"))
	}
	return b.String()
}
