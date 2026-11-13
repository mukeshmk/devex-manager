package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mukeshmk/devex-manager/internal/shared/gitx"
	"github.com/mukeshmk/devex-manager/internal/shared/testutil"
)

// cleanFix mirrors tests/test_clean.sh: stale-merged, stale-deleted and active-unmerged worktrees.
type cleanFix struct {
	f                     *testutil.Fixture
	main, merged, deleted string
	active                string
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	if _, err := gitx.Run(dir, args...); err != nil {
		t.Fatal(err)
	}
}

func newCleanFix(t *testing.T) *cleanFix {
	f := testutil.NewRepo(t)
	c := &cleanFix{f: f, main: filepath.Join(f.Root, "main"),
		merged:  f.AddWorktree(t, "stale-merged", "stale-merged"),
		deleted: f.AddWorktree(t, "stale-deleted", "stale-deleted"),
		active:  f.AddWorktree(t, "active-unmerged", "active-unmerged")}
	for _, d := range []string{c.deleted, c.active} {
		git(t, d, "commit", "-q", "--allow-empty", "-m", "work in "+d)
	}
	git(t, c.merged, "push", "-q", "-u", "origin", "stale-merged") // merged, remote branch still there
	git(t, c.deleted, "push", "-q", "-u", "origin", "stale-deleted")
	git(t, c.deleted, "push", "-q", "origin", "--delete", "stale-deleted")
	git(t, c.active, "push", "-q", "-u", "origin", "active-unmerged")
	return c
}

func clean(t *testing.T, cwd, stdin string, args ...string) (int, string) {
	t.Helper()
	t.Chdir(cwd)
	in, err := os.CreateTemp(t.TempDir(), "in")
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	in.WriteString(stdin)
	in.Seek(0, 0)
	var out bytes.Buffer
	rc := Run("git-wt", append([]string{"clean"}, args...), in, &out, &out)
	return rc, out.String()
}

func gone(t *testing.T, dir, branch string, wantGone bool) {
	t.Helper()
	_, err := gitx.Run(filepath.Dir(dir), "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	if exists(dir) == wantGone || (err != nil) != wantGone {
		t.Errorf("%s: folder exists=%v, branch err=%v; want gone=%v", branch, exists(dir), err, wantGone)
	}
}

func TestCleanRemovesStale(t *testing.T) { // tests/test_clean.sh
	c := newCleanFix(t)
	rc, out := clean(t, c.main, "y\n")
	if rc != 0 {
		t.Fatalf("rc=%d\n%s", rc, out)
	}
	for _, w := range []string{"Scanning for stale worktrees...", "Found the following stale worktrees:",
		"WARNING: This will permanently delete the worktree folder and the local branch.",
		"stale-merged (Merged into main at " + c.merged + ")", "stale-deleted (Remote branch deleted (on origin) at " + c.deleted + ")",
		"Removing worktree stale-merged...", "Removing worktree stale-deleted...", "✓ Cleanup complete!"} {
		if !strings.Contains(out, w) {
			t.Errorf("missing %q in:\n%s", w, out)
		}
	}
	if strings.Contains(out, "active-unmerged (") || strings.Contains(out, "Removing worktree active-unmerged") {
		t.Errorf("active-unmerged (upstream still on remote, unmerged) listed:\n%s", out)
	}
	gone(t, c.merged, "stale-merged", true)
	gone(t, c.deleted, "stale-deleted", true)
	gone(t, c.active, "active-unmerged", false)
	gone(t, c.main, "main", false)
}

func TestCleanSkipsDirtyByDefault(t *testing.T) {
	c := newCleanFix(t)
	write(t, filepath.Join(c.merged, "wip.txt"), "x")
	rc, out := clean(t, c.main, "y\n")
	if rc != 0 || !strings.Contains(out, "⚠ 1 uncommitted file(s), 0 commit(s) not on any remote") {
		t.Fatalf("rc=%d\n%s", rc, out)
	}
	if strings.Contains(out, "Removing worktree stale-merged") {
		t.Errorf("dirty worktree removed:\n%s", out)
	}
	gone(t, c.merged, "stale-merged", false)
	gone(t, c.deleted, "stale-deleted", true)
	gone(t, c.active, "active-unmerged", false)
}

func TestCleanNeverPushedWithCommits(t *testing.T) {
	c := newCleanFix(t)
	local := c.f.AddWorktree(t, "local", "local")
	git(t, local, "commit", "-q", "--allow-empty", "-m", "only here")
	rc, out := clean(t, c.main, "y\n")
	if rc != 0 || !strings.Contains(out, "local (No remote branch at "+local+")  ⚠ 0 uncommitted file(s), 1 commit(s) not on any remote") {
		t.Fatalf("rc=%d\n%s", rc, out)
	}
	gone(t, local, "local", false)
	gone(t, c.merged, "stale-merged", true)
	// ticking it explicitly removes it
	rc, out = clean(t, c.main, "1\n")
	if rc != 0 || !strings.Contains(out, "Removing worktree local...") {
		t.Fatalf("rc=%d\n%s", rc, out)
	}
	gone(t, local, "local", true)
	gone(t, c.active, "active-unmerged", false)
}

func TestCleanYesTakesPreselection(t *testing.T) {
	c := newCleanFix(t)
	write(t, filepath.Join(c.deleted, "wip.txt"), "x")
	rc, out := clean(t, c.main, "", "--yes")
	if rc != 0 || !strings.Contains(out, "stale-deleted (Remote branch deleted (on origin) at "+c.deleted+")") {
		t.Fatalf("rc=%d\n%s", rc, out)
	}
	gone(t, c.merged, "stale-merged", true)
	gone(t, c.deleted, "stale-deleted", false)
	gone(t, c.active, "active-unmerged", false)
}

func TestCleanAbortsOutOfSync(t *testing.T) { // tests/test_wt_deletion_sync_checks.sh TEST 3
	s := newSyncFix(t)
	outOfSync(t, s)
	rc, out := clean(t, s.main, "y\n")
	if rc == 0 || !strings.Contains(out, "Error: Cannot clean worktrees while files are not in sync.") || !strings.Contains(out, "git wt sync") {
		t.Fatalf("rc=%d\n%s", rc, out)
	}
	gone(t, s.feat, "feat", false)
	gone(t, s.feat2, "feat2", false)
}

func TestCleanEOFDeletesNothing(t *testing.T) {
	c := newCleanFix(t)
	rc, out := clean(t, c.main, "")
	if rc != 0 || !strings.Contains(out, "Cleanup cancelled.") || strings.Contains(out, "Removing worktree") {
		t.Fatalf("rc=%d\n%s", rc, out)
	}
	gone(t, c.merged, "stale-merged", false)
	gone(t, c.deleted, "stale-deleted", false)
}

func TestCleanSkipsCwd(t *testing.T) {
	c := newCleanFix(t)
	rc, out := clean(t, c.merged, "", "-y")
	if rc != 0 || !strings.Contains(out, "Error: You are currently inside the worktree 'stale-merged' ("+c.merged+") you are trying to delete!") {
		t.Fatalf("rc=%d\n%s", rc, out)
	}
	gone(t, c.merged, "stale-merged", false)
	gone(t, c.deleted, "stale-deleted", true)
}

func TestCleanNoneFound(t *testing.T) {
	f := testutil.NewRepo(t)
	rc, out := clean(t, filepath.Join(f.Root, "main"), "")
	if rc != 0 || !strings.Contains(out, "✓ No stale worktrees found.") {
		t.Fatalf("rc=%d\n%s", rc, out)
	}
}

func TestCleanRefusesWhenMainUnknown(t *testing.T) {
	c := newCleanFix(t)
	for _, d := range []string{c.main, c.merged, c.deleted, c.active} {
		os.RemoveAll(d)
	}
	rc, out := clean(t, c.f.Root, "", "-y")
	if rc == 0 || !strings.Contains(out, "No active worktrees found") {
		t.Fatalf("rc=%d\n%s", rc, out)
	}
	for _, b := range []string{"stale-merged", "stale-deleted", "active-unmerged"} {
		if _, err := gitx.Run(c.f.Root, "show-ref", "--verify", "--quiet", "refs/heads/"+b); err != nil {
			t.Errorf("branch %s deleted", b)
		}
	}
}

// M7: upstream gone but new local commits: still preselected, and the label says so.
func TestCleanLabelShowsGoneLocalCommits(t *testing.T) {
	c := newCleanFix(t)
	git(t, c.deleted, "commit", "-q", "--allow-empty", "-m", "after the remote went away") // plus the one the deleted remote branch held
	rc, out := clean(t, c.main, "", "--yes")
	if rc != 0 || !strings.Contains(out, "stale-deleted (Remote branch deleted (on origin) at "+c.deleted+")  ℹ 2 local commit(s) not on any remote") {
		t.Fatalf("rc=%d\n%s", rc, out)
	}
	gone(t, c.deleted, "stale-deleted", true)
}
