package wt

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mukeshmk/devex-manager/internal/shared/config"
	"github.com/mukeshmk/devex-manager/internal/shared/gitx"
	"github.com/mukeshmk/devex-manager/internal/shared/testutil"
)

func openFix(t *testing.T, f *testutil.Fixture) (*gitx.Repo, config.RepoConfig) {
	t.Helper()
	repo, err := gitx.Open(f.Root)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadRepo(repo.Root)
	if err != nil {
		t.Fatal(err)
	}
	return repo, cfg
}

func TestAddNewBranchFromBase(t *testing.T) {
	f := testutil.NewRepo(t)
	os.MkdirAll(filepath.Join(f.Root, "main", ".claude"), 0o755)
	os.MkdirAll(filepath.Join(f.Root, "main", "cp"), 0o755)
	src := filepath.Join(f.Root, "main", "cp", "a.txt")
	os.WriteFile(src, []byte("hi"), 0o755)
	repo, cfg := openFix(t, f)
	cfg.CopyPaths = []string{"cp"}
	var out bytes.Buffer
	p, err := Add(repo, cfg, "DEVX-9-thing", "", &out)
	if err != nil {
		t.Fatal(err)
	}
	if p != filepath.Join(f.Root, "DEVX-9") {
		t.Fatalf("path %s", p)
	}
	l, err := os.Readlink(filepath.Join(p, ".claude"))
	if err != nil || l != filepath.Join(f.Root, "main", ".claude") {
		t.Fatalf("symlink %q %v", l, err)
	}
	b, _ := os.ReadFile(filepath.Join(p, "cp", "a.txt"))
	if string(b) != "hi" {
		t.Fatalf("copy %q", b)
	}
	si, _ := os.Stat(src)
	di, _ := os.Stat(filepath.Join(p, "cp", "a.txt"))
	if si.Mode() != di.Mode() || !si.ModTime().Equal(di.ModTime()) {
		t.Fatal("copy lost mode/mtime")
	}
	for _, w := range []string{"Setting up worktree for branch: DEVX-9-thing", "Folder name will be: DEVX-9", "[1/3] Configuring git worktree...", "Creating it from 'main'", "[2/3]", "  Symlinking .claude...", "[3/3]", "  Copying cp...", `cd "` + p + `"`} {
		if !strings.Contains(out.String(), w) {
			t.Errorf("missing %q in\n%s", w, out.String())
		}
	}
}

func TestAddExistingRemoteBranch(t *testing.T) {
	f := testutil.NewRepo(t)
	bare := filepath.Join(f.Root, ".git")
	run(t, bare, "branch", "feat-x", "main")
	run(t, bare, "push", "-q", "origin", "feat-x")
	run(t, bare, "branch", "-D", "feat-x")
	run(t, bare, "fetch", "-q", "origin")
	repo, cfg := openFix(t, f)
	cfg.NamingStrategy = "full-branch"
	var out bytes.Buffer
	p, err := Add(repo, cfg, "feat-x", "", &out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "already exists. Checking it out") {
		t.Fatal(out.String())
	}
	if br, _ := gitx.Run(p, "branch", "--show-current"); br != "feat-x" {
		t.Fatal(br)
	}
}

func TestAddCustomRequiresFolder(t *testing.T) {
	f := testutil.NewRepo(t)
	repo, cfg := openFix(t, f)
	cfg.NamingStrategy = "custom"
	_, err := Add(repo, cfg, "x", "", &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "Custom folder name is required") {
		t.Fatal(err)
	}
	p, err := Add(repo, cfg, "x", "myfolder", &bytes.Buffer{})
	if err != nil || filepath.Base(p) != "myfolder" {
		t.Fatal(p, err)
	}
}

func TestRemoteBranchChoices(t *testing.T) {
	raw := "  origin/HEAD -> origin/main\n  origin/main\n  origin/feat/a\n  origin/b\n  origin/b\n"
	got := RemoteBranchChoices(raw, []gitx.Worktree{{Branch: "main"}})
	if strings.Join(got, ",") != "b,feat/a" {
		t.Fatal(got)
	}
}

func TestCloneLayout(t *testing.T) {
	f := testutil.NewRepo(t)
	cwd := filepath.Join(f.Tmp, "work")
	os.MkdirAll(cwd, 0o755)
	var out bytes.Buffer
	if err := Clone(f.Origin, cwd, &out); err != nil {
		t.Fatal(err, out.String())
	}
	d := filepath.Join(cwd, "origin")
	for _, p := range []string{".git/HEAD", "main/.git"} {
		if _, err := os.Stat(filepath.Join(d, p)); err != nil {
			t.Fatal(err)
		}
	}
	b, _ := os.ReadFile(filepath.Join(d, ".devex.conf"))
	if string(b) != strings.ReplaceAll(confTemplate, "%BRANCH%", "main") {
		t.Fatalf("conf:\n%s", b)
	}
	if !strings.Contains(string(b), "paths = .claude,.kiro,.vscode") || !strings.Contains(string(b), "base_branch = main") {
		t.Fatal("template")
	}
	if err := Clone(f.Origin, cwd, &out); err == nil || !strings.Contains(err.Error(), "already exists and is not empty") {
		t.Fatal(err)
	}
}
