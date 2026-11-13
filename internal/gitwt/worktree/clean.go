package wt

import (
	"github.com/mukeshmk/devex-manager/internal/gitwt/sync"
	"github.com/mukeshmk/devex-manager/internal/shared/config"
	"github.com/mukeshmk/devex-manager/internal/shared/gitx"
)

// Candidate is a stale worktree `git wt clean` offers to remove.
type Candidate struct {
	WT     gitx.Worktree
	Reason string
	Safety Safety
}

// FindStale lists worktrees that are merged into the base, whose upstream is gone, or that have no upstream.
// Bare, prunable, detached, base-branch and main worktrees are never candidates.
func FindStale(repo *gitx.Repo, cfg config.RepoConfig) ([]Candidate, error) {
	wts, err := repo.Worktrees()
	if err != nil {
		return nil, err
	}
	main, _, err := wtsync.MainWorktree(repo, cfg, wts)
	if err != nil {
		return nil, err
	}
	var keep []gitx.Worktree
	var branches []string
	for _, w := range wts {
		if w.Bare || w.Prunable || w.Branch == "" || w.Branch == cfg.BaseBranch || SamePath(main, w.Path) {
			continue
		}
		keep = append(keep, w)
		branches = append(branches, w.Branch)
	}
	merged, _ := repo.MergedBranches(cfg.BaseBranch) // unknown base: treat nothing as merged
	heads := RemoteHeads(repo, branches...)
	var out []Candidate
	for _, w := range keep {
		s, err := Assess(repo, cfg, w, merged, heads)
		if err != nil {
			return nil, err
		}
		c := Candidate{WT: w, Safety: s}
		switch {
		case s.Merged:
			c.Reason = "Merged into " + cfg.BaseBranch
		case s.UpstreamGone:
			remote, _ := repo.Upstream(w.Branch)
			c.Reason = "Remote branch deleted (on " + remote + ")"
		case !s.HasUpstream:
			c.Reason = "No remote branch"
		default:
			continue // upstream still exists and not merged: active work
		}
		out = append(out, c)
	}
	return out, nil
}
