package flows

import (
	"fmt"
	"strings"

	"github.com/mukeshmk/devex-manager/internal/shared/gitx"
)

// Fetch runs git fetch --all --prune in the repo root without ever reading the terminal.
func Fetch(e Env) error {
	repo, err := gitx.Open(e.Cwd)
	var out string
	if err == nil {
		out, err = gitx.RunQuiet(repo.Root, "fetch", "--all", "--prune")
		if out != "" && err == nil {
			fmt.Fprintln(e.Out, out)
		}
	}
	if err != nil {
		msg := err.Error()
		// fetch --all prints "Fetching <remote>" first; git's own last fatal/error line is the useful headline.
		lines := strings.Split(out, "\n")
		for i := len(lines) - 1; i >= 0; i-- {
			if l := strings.TrimSpace(lines[i]); strings.HasPrefix(l, "fatal:") || strings.HasPrefix(l, "error:") {
				msg = l + "\n" + msg
				break
			}
		}
		fmt.Fprintln(e.Err, colours(e.Err).red.Render(msg))
		return ErrReported
	}
	return nil
}
