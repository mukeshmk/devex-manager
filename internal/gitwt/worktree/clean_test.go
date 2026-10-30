package wt

import (
	"path/filepath"
	"testing"

	"github.com/mukeshmk/devex-manager/internal/shared/config"
	"github.com/mukeshmk/devex-manager/internal/shared/gitx"
	"github.com/mukeshmk/devex-manager/internal/shared/testutil"
)

func TestCleanFindStale(t *testing.T) {
	f := testutil.NewRepo(t)
	f.AddWorktree(t, "merged", "merged")
	gone := f.AddWorktree(t, "gone", "gone")
	active := f.AddWorktree(t, "active", "active")
	local := f.AddWorktree(t, "local", "local")
	det := f.AddWorktree(t, "det", "det")
	for _, d := range []string{gone, active, local} {
		run(t, d, "commit", "-q", "--allow-empty", "-m", "work in "+d)
	}
	run(t, gone, "push", "-q", "-u", "origin", "gone")
	run(t, gone, "push", "-q", "origin", "--delete", "gone")
	run(t, active, "push", "-q", "-u", "origin", "active")
	run(t, det, "checkout", "-q", "--detach")

	repo, err := gitx.Open(filepath.Join(f.Root, "main"))
	if err != nil {
		t.Fatal(err)
	}
	cands, err := FindStale(repo, config.RepoConfig{BaseBranch: "main", MainWorktreeName: "main"})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]Candidate{}
	for _, c := range cands {
		got[c.WT.Branch] = c
	}
	want := map[string]string{"merged": "Merged into main", "gone": "Remote branch deleted (on origin)", "local": "No remote branch"}
	if len(got) != len(want) {
		t.Fatalf("got %+v", cands)
	}
	for b, r := range want {
		if got[b].Reason != r {
			t.Errorf("%s: reason %q, want %q", b, got[b].Reason, r)
		}
	}
	if !got["merged"].Safety.Safe() || !got["gone"].Safety.Safe() || got["local"].Safety.Safe() {
		t.Errorf("safety: %+v", got)
	}
}
