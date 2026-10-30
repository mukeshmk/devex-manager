package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadRepoDefaults(t *testing.T) {
	c, err := LoadRepo(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	want := RepoConfig{BaseBranch: "main", MainWorktreeName: "main",
		SymlinkPaths: []string{".claude", ".kiro", ".vscode"}, NamingStrategy: "ticket-prefix"}
	if !reflect.DeepEqual(c, want) {
		t.Fatalf("got %+v want %+v", c, want)
	}
}

func TestLoadRepoParses(t *testing.T) {
	d := t.TempDir()
	write(t, d, ".devex.conf", "# c\n\n[repo]\n  base_branch = develop \n[copies]\npaths = .env , config/local.yaml,\n[worktree]\nnaming_strategy=full-branch\n")
	c, err := LoadRepo(d)
	if err != nil {
		t.Fatal(err)
	}
	if c.BaseBranch != "develop" || c.NamingStrategy != "full-branch" {
		t.Fatalf("got %+v", c)
	}
	if !reflect.DeepEqual(c.CopyPaths, []string{".env", "config/local.yaml"}) {
		t.Fatalf("copy paths %v", c.CopyPaths)
	}
}

func TestResolveWorktreeName(t *testing.T) {
	cases := []struct{ branch, strat, want string }{
		{"DEVX-1423-retry", "ticket-prefix", "DEVX-1423"},
		{"feature/DEVX-1-x", "ticket-prefix", "feature/DEVX-1"},
		{"a/b", "full-branch", "a--b"},
	}
	for _, c := range cases {
		got, err := ResolveWorktreeName(c.branch, c.strat)
		if err != nil || got != c.want {
			t.Errorf("%v: got %q, %v", c, got, err)
		}
	}
	if _, err := ResolveWorktreeName("x", "custom"); err == nil || !strings.Contains(err.Error(), "requires a folder name") {
		t.Errorf("custom: %v", err)
	}
	if _, err := ResolveWorktreeName("x", "bogus"); err == nil || !strings.Contains(err.Error(), "Unrecognized naming_strategy 'bogus'") {
		t.Errorf("bogus: %v", err)
	}
}

func TestLoadSkillsPrecedence(t *testing.T) {
	home := t.TempDir()
	for _, n := range []string{"a", "b", "c", "envwins"} {
		os.MkdirAll(filepath.Join(home, n), 0o755)
	}
	write(t, home, ".devex.conf", "[skills]\nmaster_dir = ~/m/\ntools = claude, kiro,gone\nclaude_dir = ~/a\nkiro_dir = ~/b\nkiro_sync_only = x,y\ngone_dir = ~/nope\nignore = z\n")
	env := map[string]string{
		"DX_CLAUDE_SKILLS_DIR": filepath.Join(home, "envwins"),
		"DX_CLAUDE_DIR":        filepath.Join(home, "c"),
	}
	c, err := LoadSkills(home, func(k string) string { return env[k] })
	if err != nil {
		t.Fatal(err)
	}
	if c.MasterDir != filepath.Join(home, "m") {
		t.Errorf("master %q", c.MasterDir)
	}
	if len(c.Tools) != 2 || c.Tools[0].Dir != filepath.Join(home, "envwins") || c.Tools[1].Dir != filepath.Join(home, "b") {
		t.Fatalf("tools %+v", c.Tools)
	}
	if !reflect.DeepEqual(c.Tools[1].SyncOnly, []string{"x", "y"}) {
		t.Errorf("sync only %v", c.Tools[1].SyncOnly)
	}
	delete(env, "DX_CLAUDE_SKILLS_DIR")
	c, _ = LoadSkills(home, func(k string) string { return env[k] })
	if c.Tools[0].Dir != filepath.Join(home, "c") {
		t.Errorf("DX_CLAUDE_DIR fallback: %+v", c.Tools[0])
	}
	if !c.IsIgnored("z") || c.IsIgnored("q") {
		t.Error("IsIgnored")
	}
}

func TestLoadSkillsWarning(t *testing.T) {
	home := t.TempDir()
	none := func(string) string { return "" }
	c, _ := LoadSkills(home, none)
	if !strings.Contains(c.Warning, "~/.devex.conf not found") {
		t.Errorf("warning %q", c.Warning)
	}
	if c.MasterDir != filepath.Join(home, "personal/skills") {
		t.Errorf("default master %q", c.MasterDir)
	}
	write(t, home, ".devex.conf", "[skills]\ntools = <tool_1>\n")
	c, _ = LoadSkills(home, none)
	if !strings.Contains(c.Warning, "has not been customized") {
		t.Errorf("warning %q", c.Warning)
	}
}
