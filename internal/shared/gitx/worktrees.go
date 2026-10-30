package gitx

import "strings"

// Worktree is one entry of `git worktree list --porcelain`.
type Worktree struct {
	Path, Branch                     string
	Bare, Detached, Locked, Prunable bool
}

// Worktrees parses `git worktree list --porcelain` line by line (paths may contain spaces).
func (r *Repo) Worktrees() ([]Worktree, error) {
	out, err := Run(r.CommonDir, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	var wts []Worktree
	for _, l := range strings.Split(out, "\n") {
		key, val, _ := strings.Cut(l, " ")
		if key == "worktree" {
			wts = append(wts, Worktree{Path: val})
			continue
		}
		if len(wts) == 0 {
			continue
		}
		w := &wts[len(wts)-1]
		switch key {
		case "branch":
			w.Branch = strings.TrimPrefix(val, "refs/heads/")
		case "bare":
			w.Bare = true
		case "detached":
			w.Detached = true
		case "locked":
			w.Locked = true
		case "prunable":
			w.Prunable = true
		}
	}
	return wts, nil
}
