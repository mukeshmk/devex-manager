package flows

import (
	"fmt"

	"github.com/mukeshmk/devex-manager/internal/shared/gitx"
)

// Fetch runs git fetch --all --prune in the repo root without ever reading the terminal.
func Fetch(e Env) error {
	repo, err := gitx.Open(e.Cwd)
	if err == nil {
		var out string
		out, err = gitx.RunQuiet(repo.Root, "fetch", "--all", "--prune")
		if out != "" && err == nil {
			fmt.Fprintln(e.Out, out)
		}
	}
	if err != nil {
		fmt.Fprintln(e.Err, colours(e.Err).red.Render(err.Error()))
		return ErrReported
	}
	return nil
}
