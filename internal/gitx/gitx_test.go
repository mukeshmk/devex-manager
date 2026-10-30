package gitx

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mukeshmk/devex-manager/internal/testutil"
)

func TestOpenBareLayout(t *testing.T) {
	f := testutil.NewRepo(t)
	r, err := Open(filepath.Join(f.Root, "main"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Root != f.Root {
		t.Errorf("Root = %q, want %q", r.Root, f.Root)
	}
	if want := filepath.Join(f.Root, ".git"); r.CommonDir != want {
		t.Errorf("CommonDir = %q, want %q", r.CommonDir, want)
	}
}

func TestWorktreesParsesOddEntries(t *testing.T) {
	f := testutil.NewRepo(t)
	det := f.AddWorktree(t, "det", "tmp-det")
	if _, err := Run(det, "checkout", "-q", "--detach"); err != nil {
		t.Fatal(err)
	}
	lk := f.AddWorktree(t, "lk", "locked-b")
	if _, err := Run(f.Root, "worktree", "lock", lk); err != nil {
		t.Fatal(err)
	}
	sp := f.AddWorktree(t, "with space", "sp")

	r, err := Open(filepath.Join(f.Root, "main"))
	if err != nil {
		t.Fatal(err)
	}
	wts, err := r.Worktrees()
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]Worktree{}
	for _, w := range wts {
		by[w.Path] = w
	}
	if w := by[f.Root+"/.git"]; !w.Bare {
		// bare entry path is the git dir's parent or the dir itself, depending on git
		found := false
		for _, w := range wts {
			if w.Bare {
				found = true
			}
		}
		if !found {
			t.Errorf("no bare entry in %+v", wts)
		}
	}
	if w := by[det]; !w.Detached || w.Branch != "" {
		t.Errorf("detached = %+v", w)
	}
	if w := by[lk]; !w.Locked || w.Branch != "locked-b" {
		t.Errorf("locked = %+v", w)
	}
	if w, ok := by[sp]; !ok || w.Branch != "sp" {
		t.Errorf("spaced = %+v ok=%v in %+v", w, ok, wts)
	}
}

func TestRemoteHeadsSingleCall(t *testing.T) {
	f := testutil.NewRepo(t)
	wt := f.AddWorktree(t, "a", "a")
	if _, err := Run(wt, "push", "-q", "origin", "a"); err != nil {
		t.Fatal(err)
	}
	r, err := Open(wt)
	if err != nil {
		t.Fatal(err)
	}
	h, err := r.RemoteHeads("origin")
	if err != nil {
		t.Fatal(err)
	}
	if len(h) != 2 || !h["main"] || !h["a"] {
		t.Errorf("heads = %v", h)
	}
}

func TestHelpers(t *testing.T) {
	f := testutil.NewRepo(t)
	main := filepath.Join(f.Root, "main")
	r, _ := Open(main)
	if rem, ok := r.Upstream("main"); ok {
		t.Errorf("unexpected upstream %q", rem)
	}
	if _, _, has, err := AheadBehind(main, "main"); err != nil || has {
		t.Errorf("AheadBehind has=%v err=%v", has, err)
	}
	wt := f.AddWorktree(t, "b", "b")
	if err := os.WriteFile(filepath.Join(wt, "x.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if files, err := ChangedFiles(wt); err != nil || len(files) != 1 {
		t.Errorf("ChangedFiles = %v, %v", files, err)
	}
	for _, c := range [][]string{{"add", "x.txt"}, {"commit", "-q", "-m", "x"}} {
		if out, err := exec.Command("git", append([]string{"-C", wt}, c...)...).CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}
	if n, err := UnpushedCount(wt, "b"); err != nil || n != 1 {
		t.Errorf("UnpushedCount = %d, %v", n, err)
	}
	m, err := r.MergedBranches("main")
	if err != nil || !m["main"] || m["b"] {
		t.Errorf("merged = %v, %v", m, err)
	}
}
