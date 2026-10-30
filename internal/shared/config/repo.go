package config

import (
	"fmt"
	"path/filepath"
	"strings"
)

type RepoConfig struct {
	BaseBranch, MainWorktreeName string
	SymlinkPaths, CopyPaths      []string
	NamingStrategy               string
}

// LoadRepo reads <repoRoot>/.devex.conf; a missing file yields defaults.
func LoadRepo(repoRoot string) (RepoConfig, error) {
	ini, err := readINI(filepath.Join(repoRoot, ".devex.conf"))
	if err != nil {
		return RepoConfig{}, err
	}
	get := func(sec, key, def string) string {
		if v, ok := ini[sec][key]; ok {
			return v
		}
		return def
	}
	return RepoConfig{
		BaseBranch:       get("repo", "base_branch", "main"),
		MainWorktreeName: get("repo", "main_worktree_name", "main"),
		SymlinkPaths:     splitList(get("symlinks", "paths", ".claude,.kiro,.vscode")),
		CopyPaths:        splitList(get("copies", "paths", "")),
		NamingStrategy:   get("worktree", "naming_strategy", "ticket-prefix"),
	}, nil
}

// ResolveWorktreeName maps a branch to a worktree folder name.
func ResolveWorktreeName(branch, strategy string) (string, error) {
	switch strategy {
	case "ticket-prefix":
		// parity with `cut -d- -f1,2`
		f := strings.Split(branch, "-")
		if len(f) > 2 {
			f = f[:2]
		}
		return strings.Join(f, "-"), nil
	case "full-branch":
		return strings.ReplaceAll(branch, "/", "--"), nil
	case "custom":
		return "", fmt.Errorf("Error: 'custom' naming strategy requires a folder name argument")
	}
	return "", fmt.Errorf("Error: Unrecognized naming_strategy '%s' in .devex.conf", strategy)
}
