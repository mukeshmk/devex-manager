// Package wt holds the worktree deletion rules shared by `git wt rm`, `git wt clean` and the app.
package wt

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/mukeshmk/devex-manager/internal/gitx"
)

// Safety is what deleting a worktree and its branch would lose.
type Safety struct {
	Uncommitted                       []string
	Unpushed                          int
	Merged, UpstreamGone, HasUpstream bool
}

// Safe is the owner-approved rule: clean tree, and merged, upstream gone, or nothing unpushed.
func (s Safety) Safe() bool {
	return len(s.Uncommitted) == 0 && (s.Merged || s.UpstreamGone || s.Unpushed == 0)
}

// Assess inspects wt; remoteHeads is keyed by remote name, as fetched by the caller.
// A missing remote entry (e.g. ls-remote failed) means the upstream is not treated as gone.
func Assess(repo *gitx.Repo, wt gitx.Worktree, merged map[string]bool, remoteHeads map[string]map[string]bool) (Safety, error) {
	var s Safety
	var err error
	present := wt.Path != "" && !wt.Prunable
	if present {
		if s.Uncommitted, err = gitx.ChangedFiles(wt.Path); err != nil {
			return s, err
		}
	}
	ref, dir := wt.Branch, repo.CommonDir
	if ref == "" { // detached HEAD: count from the worktree itself
		if !present {
			return s, nil
		}
		ref, dir = "HEAD", wt.Path
	}
	if s.Unpushed, err = gitx.UnpushedCount(dir, ref); err != nil {
		return s, err
	}
	if wt.Branch != "" {
		s.Merged = merged[wt.Branch]
		var remote string
		remote, s.HasUpstream = repo.Upstream(wt.Branch)
		// Gone means the upstream's own remote branch (branch.<b>.merge) is missing; a "." remote is local.
		merge, mErr := gitx.Run(repo.CommonDir, "config", "--get", "branch."+wt.Branch+".merge")
		heads, fetched := remoteHeads[remote]
		s.UpstreamGone = s.HasUpstream && remote != "." && mErr == nil && merge != "" && fetched &&
			!heads[strings.TrimPrefix(merge, "refs/heads/")]
	}
	return s, nil
}

// Inside reports whether cwd is path or below it, comparing whole path components.
func Inside(cwd, path string) bool {
	cwd, path = canon(cwd), canon(path)
	return cwd == path || strings.HasPrefix(cwd, path+string(os.PathSeparator))
}

// canon resolves symlinks, falling back to Clean when the path is gone.
func canon(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return filepath.Clean(p)
}

// SamePath compares two paths after symlink resolution.
func SamePath(a, b string) bool { return canon(a) == canon(b) }
