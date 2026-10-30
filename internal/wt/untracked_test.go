package wt

import (
	"testing"

	"github.com/mukeshmk/devex-manager/internal/config"
)

// Files sync never copies must stay "dirty" even under a copy path, or rm would delete them unseen.
func TestConfiguredUntrackedKeepsUnsyncedNames(t *testing.T) {
	cfg := config.RepoConfig{CopyPaths: []string{".claude"}}
	cases := map[string]bool{
		"?? .claude/settings.json":                 true,
		"?? .claude/notes.devex-conflict-mine.md":  false,
		"?? .claude/.settings.json.devex-tmp-1234": false,
		"?? .claudex/notes.md":                     false,
	}
	for line, want := range cases {
		if got := configuredUntracked(line, t.TempDir(), cfg); got != want {
			t.Errorf("%q: got %v, want %v", line, got, want)
		}
	}
}
