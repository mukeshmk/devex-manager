package wt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/mukeshmk/devex-manager/internal/shared/config"
	"github.com/mukeshmk/devex-manager/internal/shared/gitx"
	"github.com/mukeshmk/devex-manager/internal/shared/testutil"
)

func collect(t *testing.T, f *testutil.Fixture) []Row {
	t.Helper()
	repo, err := gitx.Open(f.Root)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadRepo(repo.Root)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := Collect(repo, cfg)
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func TestStatusDirtyAndClean(t *testing.T) {
	f := testutil.NewRepo(t)
	feat := f.AddWorktree(t, "feat", "feat")
	os.WriteFile(filepath.Join(feat, "README.md"), []byte("x"), 0o644)
	rows := collect(t, f)
	if len(rows) != 2 { // bare entry skipped
		t.Fatalf("want 2 rows, got %d", len(rows))
	}
	out := RenderTable(rows, 140)
	for _, w := range []string{"feat", "● 1 files", "○ Clean"} {
		if !strings.Contains(out, w) {
			t.Errorf("missing %q in\n%s", w, out)
		}
	}
	if !strings.Contains(out, "Total Worktrees: 2") {
		t.Errorf("bad summary:\n%s", out)
	}
}

func TestStatusDetachedAndAhead(t *testing.T) {
	f := testutil.NewRepo(t)
	a := f.AddWorktree(t, "ahead", "ahead")
	run(t, a, "push", "-q", "-u", "origin", "ahead")
	run(t, a, "commit", "-q", "--allow-empty", "-m", "more")
	run(t, filepath.Join(f.Root, "main"), "worktree", "add", "-q", "--detach", filepath.Join(f.Root, "det"))
	rows := collect(t, f)
	byName := map[string]Row{}
	for _, r := range rows {
		byName[r.Folder] = r
	}
	if r := byName["ahead"]; !r.HasUpstream || r.Ahead != 1 || r.Behind != 0 {
		t.Errorf("ahead row: %+v", r)
	}
	if !byName["det"].WT.Detached {
		t.Errorf("det not detached")
	}
	out := RenderTable(rows, 140)
	for _, w := range []string{"↑1", "N/A", "(detached)", "No upstream"} {
		if !strings.Contains(out, w) {
			t.Errorf("missing %q in\n%s", w, out)
		}
	}
}

func TestStatusMergedStale(t *testing.T) {
	f := testutil.NewRepo(t)
	f.AddWorktree(t, "done", "done") // same commit as main: merged
	out := RenderTable(collect(t, f), 140)
	if !strings.Contains(out, "Stale:") || !strings.Contains(out, "1 Merged") {
		t.Errorf("want stale line:\n%s", out)
	}
}

func TestRenderTableWidths(t *testing.T) {
	f := testutil.NewRepo(t)
	f.AddWorktree(t, "DEVX-1234-a-really-long-worktree-folder", "feature/DEVX-1234-a-really-long-branch-name")
	rows := collect(t, f)
	for _, w := range []int{60, 160} {
		for _, l := range strings.Split(RenderTable(rows, w), "\n") {
			if ansi.StringWidth(l) > w {
				t.Errorf("width %d: line %d wide: %q", w, ansi.StringWidth(l), l)
			}
		}
	}
	if !strings.Contains(RenderTable(rows, 160), "20") { // absolute timestamp
		t.Error("expected absolute date at 160")
	}
	if !strings.Contains(RenderTable(rows, 100), " ago") && !strings.Contains(RenderTable(rows, 100), "just now") {
		t.Error("expected relative time at 100")
	}
}

func TestStatusPrunableError(t *testing.T) {
	f := testutil.NewRepo(t)
	p := f.AddWorktree(t, "gone", "gone")
	os.RemoveAll(p)
	var got *Row
	rows := collect(t, f)
	for i := range rows {
		if rows[i].Folder == "gone" {
			got = &rows[i]
		}
	}
	if got == nil || got.Err == nil || !strings.Contains(RenderTable(rows, 140), "Error") {
		t.Fatalf("want Error row: %+v", got)
	}
}
