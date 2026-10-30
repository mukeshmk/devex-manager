package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mukeshmk/devex-manager/internal/gitx"
)

func (s *syncFix) rm(t *testing.T, args ...string) (int, string) {
	t.Helper()
	t.Chdir(s.main)
	in, err := os.CreateTemp(t.TempDir(), "in")
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	var out bytes.Buffer
	rc := Run("git-wt", append([]string{"rm"}, args...), in, &out, &out)
	return rc, out.String()
}

func exists(p string) bool { _, err := os.Lstat(p); return err == nil }

func branchExists(t *testing.T, s *syncFix, b string) bool {
	_, err := gitx.Run(s.main, "show-ref", "--verify", "--quiet", "refs/heads/"+b)
	return err == nil
}

// outOfSync mirrors tests/test_wt_deletion_sync_checks.sh: a copy path differs between worktrees.
func outOfSync(t *testing.T, s *syncFix) {
	write(t, filepath.Join(s.main, ".vscode", "a.json"), "main")
	write(t, filepath.Join(s.feat, ".vscode", "a.json"), "feat")
}

func TestRmAbortsWhenOutOfSync(t *testing.T) { // TEST 4
	s := newSyncFix(t)
	outOfSync(t, s)
	rc, out := s.rm(t, "feat")
	if rc == 0 || !strings.Contains(out, "Cannot remove worktree while files are not in sync") || !strings.Contains(out, "git wt sync") {
		t.Fatalf("rc=%d out=%s", rc, out)
	}
	if !exists(s.feat) || !branchExists(t, s, "feat") {
		t.Fatal("refusal changed something")
	}
}

func TestRmForceBypassesSync(t *testing.T) { // TEST 5-7
	for _, args := range [][]string{{"-f", "feat"}, {"--force", "feat"}, {"feat", "-f"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			s := newSyncFix(t)
			outOfSync(t, s)
			os.Symlink(s.main, filepath.Join(s.feat, ".claude"))
			rc, out := s.rm(t, args...)
			if rc != 0 || strings.Contains(out, "not in sync") {
				t.Fatalf("rc=%d out=%s", rc, out)
			}
			for _, want := range []string{"Cleaning up worktree and branch for: feat", "[1/2] Removing worktree folder and configuration...",
				"Removing symlink .claude...", "[2/2] Deleting local branch...", "✓ Clean up complete! Workspace is tidy."} {
				if !strings.Contains(out, want) {
					t.Errorf("missing %q in:\n%s", want, out)
				}
			}
			if exists(s.feat) || branchExists(t, s, "feat") {
				t.Fatal("not removed")
			}
			if !exists(s.main) {
				t.Fatal("symlink target was removed")
			}
		})
	}
}

func TestRmUsageErrors(t *testing.T) { // TEST 8-9
	s := newSyncFix(t)
	if rc, out := s.rm(t, "-f"); rc == 0 || !strings.Contains(out, "Error: Target (folder or branch name) is required") || !strings.Contains(out, "Usage: git wt rm [-f|--force] <folder-name-or-branch-name>") {
		t.Errorf("no target: rc=%d out=%s", rc, out)
	}
	if rc, out := s.rm(t, "-f", "feat", "another-arg"); rc == 0 || !strings.Contains(out, "Error: Too many arguments specified") {
		t.Errorf("too many: rc=%d out=%s", rc, out)
	}
	if !exists(s.feat) {
		t.Fatal("usage error removed worktree")
	}
}

func TestRmRefusesDirty(t *testing.T) {
	s := newSyncFix(t)
	for i := 0; i < 12; i++ {
		write(t, filepath.Join(s.feat, "f"+string(rune('a'+i))+".txt"), "x")
	}
	if _, err := gitx.Run(s.feat2, "commit", "-q", "--allow-empty", "-m", "local"); err != nil {
		t.Fatal(err)
	}
	rc, out := s.rm(t, "feat")
	if rc == 0 || !strings.Contains(out, "?? fa.txt") || !strings.Contains(out, "and 2 more") || !strings.Contains(out, "use --force to remove anyway") {
		t.Fatalf("dirty: rc=%d out=%s", rc, out)
	}
	if !exists(s.feat) || !branchExists(t, s, "feat") {
		t.Fatal("refusal changed something")
	}
	rc, out = s.rm(t, "feat2")
	if rc == 0 || !strings.Contains(out, "1 commit(s) not on any remote") {
		t.Fatalf("unpushed: rc=%d out=%s", rc, out)
	}
	if !exists(s.feat2) || !branchExists(t, s, "feat2") {
		t.Fatal("refusal changed something")
	}
	if rc, out = s.rm(t, "--force", "feat"); rc != 0 || exists(s.feat) {
		t.Fatalf("force: rc=%d out=%s", rc, out)
	}
}

func TestRmRefusesMainAndBase(t *testing.T) {
	s := newSyncFix(t)
	fromRoot := *s
	fromRoot.main = s.f.Root // not inside main, so the main rule is what refuses
	if rc, out := fromRoot.rm(t, "--force", "main"); rc == 0 || !strings.Contains(out, "Refusing to remove the main worktree 'main'") {
		t.Errorf("main: rc=%d out=%s", rc, out)
	}
	// base branch refused even with --force
	write(t, filepath.Join(s.f.Root, ".devex.conf"), "[repo]\nbase_branch = feat2\n")
	if rc, out := s.rm(t, "-f", "feat2"); rc == 0 || !strings.Contains(out, "Refusing to delete the base branch 'feat2'") {
		t.Errorf("base: rc=%d out=%s", rc, out)
	}
	if !exists(s.main) || !exists(s.feat2) || !branchExists(t, s, "main") {
		t.Fatal("refusal changed something")
	}
}

func TestRmRefusesInside(t *testing.T) {
	s := newSyncFix(t)
	s.main = s.feat // run from inside the target
	rc, out := s.rm(t, "-f", "feat")
	if rc == 0 || !strings.Contains(out, "You are currently inside the worktree you are trying to delete!") {
		t.Fatalf("rc=%d out=%s", rc, out)
	}
	if !exists(s.feat) || !branchExists(t, s, "feat") {
		t.Fatal("refusal changed something")
	}
}

// A detached worktree in folder feat plus a branch feat: rm deletes both, so both must be safe.
func TestRmDetachedGuardsSameNamedBranch(t *testing.T) {
	s := newSyncFix(t)
	for _, args := range [][]string{{"commit", "-q", "--allow-empty", "-m", "local-only"}, {"checkout", "-q", "--detach", "HEAD~1"}} {
		if _, err := gitx.Run(s.feat, args...); err != nil {
			t.Fatal(err)
		}
	}
	rc, out := s.rm(t, "feat")
	if rc == 0 || !strings.Contains(out, "1 commit(s) not on any remote") {
		t.Fatalf("rc=%d out=%s", rc, out)
	}
	if !exists(s.feat) || !branchExists(t, s, "feat") {
		t.Fatal("refusal changed something")
	}
	if rc, out = s.rm(t, "--force", "feat"); rc != 0 || exists(s.feat) || branchExists(t, s, "feat") {
		t.Fatalf("force: rc=%d out=%s", rc, out)
	}
}

// If the main worktree cannot be identified, rm must refuse rather than skip the main guard.
func TestRmRefusesWhenMainUnknown(t *testing.T) {
	s := newSyncFix(t)
	for _, d := range []string{s.main, s.feat, s.feat2} {
		os.RemoveAll(d) // every worktree prunable: MainWorktree errors
	}
	fromRoot := *s
	fromRoot.main = s.f.Root
	rc, out := fromRoot.rm(t, "--force", "feat")
	if rc == 0 || !strings.Contains(out, "No active worktrees found") {
		t.Fatalf("rc=%d out=%s", rc, out)
	}
	if !branchExists(t, &fromRoot, "feat") {
		t.Fatal("refusal changed something")
	}
}

func TestRmBranchOnly(t *testing.T) {
	s := newSyncFix(t)
	if _, err := gitx.Run(s.main, "branch", "DEVX-7-orphan"); err != nil {
		t.Fatal(err)
	}
	rc, out := s.rm(t, "DEVX-7-orphan")
	if rc != 0 || !strings.Contains(out, "Worktree for 'DEVX-7-orphan' not found") || branchExists(t, s, "DEVX-7-orphan") {
		t.Fatalf("rc=%d out=%s", rc, out)
	}
	rc, out = s.rm(t, "nothing")
	if rc == 0 || !strings.Contains(out, "Local branch 'nothing' does not exist.") {
		t.Fatalf("none: rc=%d out=%s", rc, out)
	}
}

// I1: a prefix is never silently widened to a branch the user did not name.
func TestRmPrefixOnlyRefuses(t *testing.T) {
	s := newSyncFix(t)
	for _, b := range []string{"release-2026", "release-2027"} {
		if _, err := gitx.Run(s.main, "branch", b); err != nil {
			t.Fatal(err)
		}
	}
	rc, out := s.rm(t, "rel")
	if rc != 1 || !strings.Contains(out, `No worktree or branch named "rel". Did you mean: release-2026, release-2027?`) {
		t.Fatalf("rc=%d out=%s", rc, out)
	}
	if !branchExists(t, s, "release-2026") || !branchExists(t, s, "release-2027") {
		t.Fatal("prefix match deleted a branch")
	}
}

// C1: folder alpha holds branch beta, folder beta holds branch gamma; rm beta removes folder beta only.
func TestRmFolderBeatsBranch(t *testing.T) {
	s := newSyncFix(t)
	alpha := s.f.AddWorktree(t, "alpha", "beta")
	beta := s.f.AddWorktree(t, "beta", "gamma")
	rc, out := s.rm(t, "beta")
	if rc != 0 || !strings.Contains(out, "Cleaning up worktree and branch for: gamma") {
		t.Fatalf("rc=%d out=%s", rc, out)
	}
	if exists(beta) || branchExists(t, s, "gamma") {
		t.Fatal("folder beta / branch gamma not removed")
	}
	if !exists(alpha) || !branchExists(t, s, "beta") {
		t.Fatal("removed the wrong worktree")
	}
	// the app passes an absolute path
	if rc, out = s.rm(t, alpha); rc != 0 || exists(alpha) || branchExists(t, s, "beta") {
		t.Fatalf("abs path: rc=%d out=%s", rc, out)
	}
}

// I2: untracked copy/symlink paths left by add + sync do not make a worktree unsafe.
func TestRmIgnoresUntrackedConfiguredPaths(t *testing.T) {
	s := newSyncFix(t)
	write(t, filepath.Join(s.f.Root, ".devex.conf"), "[copies]\npaths = .vscode\n[symlinks]\npaths = .claude\n")
	for _, w := range []string{s.main, s.feat, s.feat2} {
		write(t, filepath.Join(w, ".vscode", "settings.json"), "{}")
	}
	os.Symlink(filepath.Join(s.main, ".claude"), filepath.Join(s.feat, ".claude"))
	if rc, out := s.run(t); rc != 0 {
		t.Fatalf("sync: rc=%d out=%s", rc, out)
	}
	if rc, out := s.rm(t, "feat"); rc != 0 || exists(s.feat) {
		t.Fatalf("rm clean worktree: rc=%d out=%s", rc, out)
	}
	write(t, filepath.Join(s.feat2, "real.txt"), "x")
	if rc, out := s.rm(t, "feat2"); rc == 0 || !strings.Contains(out, "?? real.txt") || strings.Contains(out, ".vscode") {
		t.Fatalf("real untracked file: rc=%d out=%s", rc, out)
	}
}
