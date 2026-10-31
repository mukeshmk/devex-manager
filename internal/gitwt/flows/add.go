package flows

import (
	"fmt"

	"github.com/mukeshmk/devex-manager/internal/gitwt/worktree"
	"github.com/mukeshmk/devex-manager/internal/shared/config"
	"github.com/mukeshmk/devex-manager/internal/shared/gitx"
)

// Add runs git wt add for a known branch. Failures are printed to e.Err and returned as ErrReported.
func Add(e Env, branch, folder string) error {
	repo, err := gitx.Open(e.Cwd)
	var cfg config.RepoConfig
	if err == nil {
		cfg, err = config.LoadRepo(repo.Root)
	}
	if err == nil {
		_, err = wt.Add(repo, cfg, branch, folder, e.Out)
	}
	if err != nil {
		fmt.Fprintln(e.Err, colours(e.Err).red.Render(err.Error()))
		return ErrReported
	}
	return nil
}

// RemoteBranches lists origin branches that have no worktree yet, sorted.
func RemoteBranches(cwd string) ([]string, error) {
	repo, err := gitx.Open(cwd)
	if err != nil {
		return nil, err
	}
	raw, err := gitx.Run(repo.CommonDir, "branch", "-r")
	if err != nil {
		return nil, err
	}
	wts, err := repo.Worktrees()
	if err != nil {
		return nil, err
	}
	return wt.RemoteBranchChoices(raw, wts), nil
}
