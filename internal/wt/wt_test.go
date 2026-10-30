package wt

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mukeshmk/devex-manager/internal/gitx"
	"github.com/mukeshmk/devex-manager/internal/testutil"
)

func TestInsidePrefixCollision(t *testing.T) {
	r := t.TempDir()
	for _, d := range []string{"DEVX-1/sub", "DEVX-12"} {
		os.MkdirAll(filepath.Join(r, d), 0o755)
	}
	cases := []struct {
		cwd, path string
		want      bool
	}{
		{"DEVX-12", "DEVX-1", false},
		{"DEVX-1/sub", "DEVX-1", true},
		{"DEVX-1", "DEVX-1", true},
		{"DEVX-1", "gone", false}, // target dir missing: Clean fallback
	}
	for _, c := range cases {
		if got := Inside(filepath.Join(r, c.cwd), filepath.Join(r, c.path)); got != c.want {
			t.Errorf("Inside(%s, %s) = %v, want %v", c.cwd, c.path, got, c.want)
		}
	}
}

func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	if _, err := gitx.Run(dir, args...); err != nil {
		t.Fatal(err)
	}
}

func assess(t *testing.T, f *testutil.Fixture, branch string) Safety {
	t.Helper()
	repo, err := gitx.Open(filepath.Join(f.Root, "main"))
	if err != nil {
		t.Fatal(err)
	}
	wts, _ := repo.Worktrees()
	w, _ := FindTarget(wts, repo, branch)
	if w == nil {
		t.Fatalf("no worktree for %s", branch)
	}
	merged, _ := repo.MergedBranches("main")
	heads, _ := repo.RemoteHeads("origin")
	s, err := Assess(repo, *w, merged, map[string]map[string]bool{"origin": heads})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSafeMatrix(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, f *testutil.Fixture, wt string)
		safe  bool
		gone  bool // want UpstreamGone
	}{
		{"dirty tree", func(t *testing.T, f *testutil.Fixture, wt string) {
			os.WriteFile(filepath.Join(wt, "new.txt"), []byte("x"), 0o644)
		}, false, false},
		{"never pushed, 1 commit", func(t *testing.T, f *testutil.Fixture, wt string) {
			run(t, wt, "commit", "-q", "--allow-empty", "-m", "c1")
		}, false, false},
		{"never pushed, 0 commits", func(t *testing.T, f *testutil.Fixture, wt string) {}, true, false},
		{"upstream gone", func(t *testing.T, f *testutil.Fixture, wt string) {
			run(t, wt, "commit", "-q", "--allow-empty", "-m", "c1")
			run(t, wt, "push", "-q", "-u", "origin", "feat")
			run(t, f.Origin, "branch", "-D", "feat")
			run(t, wt, "update-ref", "-d", "refs/remotes/origin/feat")
			run(t, wt, "commit", "-q", "--allow-empty", "-m", "squashed-elsewhere")
		}, true, true},
		{"upstream is origin/main, 1 local commit", func(t *testing.T, f *testutil.Fixture, wt string) {
			run(t, wt, "branch", "-q", "--set-upstream-to=origin/main")
			run(t, wt, "commit", "-q", "--allow-empty", "-m", "c1")
		}, false, false},
		{"upstream is local main, 1 local commit", func(t *testing.T, f *testutil.Fixture, wt string) {
			run(t, wt, "branch", "-q", "--set-upstream-to=main")
			run(t, wt, "commit", "-q", "--allow-empty", "-m", "c1")
		}, false, false},
		{"pushed as other, other gone", func(t *testing.T, f *testutil.Fixture, wt string) {
			run(t, wt, "commit", "-q", "--allow-empty", "-m", "c1")
			run(t, wt, "push", "-q", "-u", "origin", "feat:other")
			run(t, f.Origin, "branch", "-D", "other")
			run(t, wt, "update-ref", "-d", "refs/remotes/origin/other")
			run(t, wt, "commit", "-q", "--allow-empty", "-m", "squashed-elsewhere")
		}, true, true},
		{"merged", func(t *testing.T, f *testutil.Fixture, wt string) {
			run(t, wt, "commit", "-q", "--allow-empty", "-m", "c1")
			run(t, filepath.Join(f.Root, "main"), "merge", "-q", "--ff-only", "feat")
		}, true, false},
		{"upstream exists, ahead 1", func(t *testing.T, f *testutil.Fixture, wt string) {
			run(t, wt, "commit", "-q", "--allow-empty", "-m", "c1")
			run(t, wt, "push", "-q", "-u", "origin", "feat")
			run(t, wt, "commit", "-q", "--allow-empty", "-m", "c2")
		}, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := testutil.NewRepo(t)
			wt := f.AddWorktree(t, "feat", "feat")
			c.setup(t, f, wt)
			s := assess(t, f, "feat")
			if s.Safe() != c.safe || s.UpstreamGone != c.gone {
				t.Errorf("Safe() = %v, want %v; UpstreamGone want %v (%+v)", s.Safe(), c.safe, c.gone, s)
			}
		})
	}
}

func TestAssessFlags(t *testing.T) {
	f := testutil.NewRepo(t)
	wt := f.AddWorktree(t, "feat", "feat")
	run(t, wt, "commit", "-q", "--allow-empty", "-m", "c1")
	if s := assess(t, f, "feat"); s.HasUpstream || s.Unpushed != 1 || s.Merged || s.UpstreamGone {
		t.Errorf("never pushed: %+v", s)
	}
	run(t, wt, "push", "-q", "-u", "origin", "feat")
	if s := assess(t, f, "feat"); !s.HasUpstream || s.Unpushed != 0 || s.UpstreamGone {
		t.Errorf("pushed: %+v", s)
	}
}

func TestFindTarget(t *testing.T) {
	f := testutil.NewRepo(t)
	f.AddWorktree(t, "DEVX-1", "DEVX-1-thing")
	run(t, filepath.Join(f.Root, "main"), "branch", "DEVX-9-orphan")
	repo, _ := gitx.Open(filepath.Join(f.Root, "main"))
	wts, _ := repo.Worktrees()
	if w, b := FindTarget(wts, repo, "DEVX-1"); w == nil || b != "DEVX-1-thing" {
		t.Errorf("folder match: %v %q", w, b)
	}
	if w, b := FindTarget(wts, repo, "DEVX-1-thing"); w == nil || b != "DEVX-1-thing" {
		t.Errorf("branch match: %v %q", w, b)
	}
	if w, b := FindTarget(wts, repo, "DEVX-9"); w != nil || b != "DEVX-9-orphan" {
		t.Errorf("prefix fallback: %v %q", w, b)
	}
	if w, b := FindTarget(wts, repo, "nope"); w != nil || b != "nope" {
		t.Errorf("none: %v %q", w, b)
	}
}
