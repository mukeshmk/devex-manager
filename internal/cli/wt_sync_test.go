package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mukeshmk/devex-manager/internal/testutil"
)

type syncFix struct {
	f                 *testutil.Fixture
	main, feat, feat2 string
	manifest          string
}

func newSyncFix(t *testing.T) *syncFix {
	f := testutil.NewRepo(t)
	s := &syncFix{f: f, main: filepath.Join(f.Root, "main"),
		feat: f.AddWorktree(t, "feat", "feat"), feat2: f.AddWorktree(t, "feat2", "feat2"),
		manifest: filepath.Join(f.Root, ".git", "info", "devex", "sync-manifest")}
	write(t, filepath.Join(f.Root, ".devex.conf"), "[copies]\npaths = .vscode, my dir/conf.json\n")
	return s
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// touchLater writes content with an mtime that is clearly newer than any manifest entry.
func touchLater(t *testing.T, path, content string) {
	write(t, path, content)
	later := time.Now().Add(time.Hour)
	os.Chtimes(path, later, later)
}

func (s *syncFix) run(t *testing.T, args ...string) (int, string) {
	t.Helper()
	t.Chdir(s.main)
	in, err := os.CreateTemp(t.TempDir(), "in")
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	var out bytes.Buffer
	rc := Run("git-wt", append([]string{"sync"}, args...), in, &out, &out)
	return rc, out.String()
}

func fileIs(t *testing.T, path, want string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil || string(b) != want {
		t.Fatalf("%s = %q (%v), want %q", path, b, err, want)
	}
}

func TestSyncEndToEnd(t *testing.T) {
	s := newSyncFix(t)
	write(t, filepath.Join(s.feat, ".vscode/settings.json"), "{}")
	write(t, filepath.Join(s.main, "my dir/conf.json"), "conf")
	rc, out := s.run(t)
	if rc != 0 {
		t.Fatalf("rc=%d\n%s", rc, out)
	}
	for _, w := range []string{"Synchronizing configuration copies...", "Main worktree: main", "update .vscode/settings.json in main, feat2", "✓ Synchronization complete!"} {
		if !strings.Contains(out, w) {
			t.Errorf("output missing %q:\n%s", w, out)
		}
	}
	if strings.Contains(out, "Backups stored in") {
		t.Errorf("no file was overwritten, but backups reported:\n%s", out)
	}
	fileIs(t, filepath.Join(s.main, ".vscode/settings.json"), "{}")
	fileIs(t, filepath.Join(s.feat2, ".vscode/settings.json"), "{}")
	fileIs(t, filepath.Join(s.feat2, "my dir/conf.json"), "conf")
	m, _ := os.ReadFile(s.manifest)
	if !strings.Contains(string(m), "my dir/conf.json|") || !strings.Contains(string(m), ".vscode/settings.json|") {
		t.Fatalf("manifest:\n%s", m)
	}
	// second run: in sync, manifest still rebuilt
	if rc, out = s.run(t); rc != 0 || !strings.Contains(out, "✓ Everything is in sync.") {
		t.Fatalf("rc=%d\n%s", rc, out)
	}
}

func TestSyncFailureKeepsManifest(t *testing.T) {
	s := newSyncFix(t)
	write(t, filepath.Join(s.main, ".vscode/settings.json"), "v1")
	if rc, out := s.run(t); rc != 0 {
		t.Fatalf("setup rc=%d\n%s", rc, out)
	}
	before, err := os.ReadFile(s.manifest)
	if err != nil {
		t.Fatal(err)
	}
	touchLater(t, filepath.Join(s.feat, ".vscode/settings.json"), "v2")
	ro := filepath.Join(s.feat2, ".vscode")
	os.Chmod(ro, 0o555)
	t.Cleanup(func() { os.Chmod(ro, 0o755) })
	rc, out := s.run(t)
	if rc != 1 {
		t.Fatalf("rc=%d\n%s", rc, out)
	}
	if !strings.Contains(out, "Backups stored in: "+filepath.Join(s.f.Root, ".git", "info", "devex", "sync-backups")) {
		t.Errorf("backup dir not reported:\n%s", out)
	}
	if after, _ := os.ReadFile(s.manifest); !bytes.Equal(before, after) {
		t.Fatalf("manifest rewritten after failure:\n%s\nvs\n%s", before, after)
	}
	fileIs(t, filepath.Join(s.feat2, ".vscode/settings.json"), "v1")
}

func TestSyncCheckExitCodes(t *testing.T) {
	s := newSyncFix(t)
	write(t, filepath.Join(s.main, ".vscode/settings.json"), "v1")
	if rc, out := s.run(t); rc != 0 {
		t.Fatalf("setup rc=%d\n%s", rc, out)
	}
	rc, out := s.run(t, "--check")
	if rc != 0 || !strings.Contains(out, "✓ Everything is in sync.") || !strings.Contains(out, "Checking sync status of configuration copies...") {
		t.Fatalf("rc=%d\n%s", rc, out)
	}
	touchLater(t, filepath.Join(s.feat, ".vscode/settings.json"), "v2")
	rc, out = s.run(t, "-c")
	if rc != 1 || !strings.Contains(out, "Out of sync: .vscode/settings.json") || !strings.Contains(out, "Modified in: feat") {
		t.Fatalf("rc=%d\n%s", rc, out)
	}
	fileIs(t, filepath.Join(s.main, ".vscode/settings.json"), "v1")
}

func TestSyncDryRunWritesNothing(t *testing.T) {
	s := newSyncFix(t)
	write(t, filepath.Join(s.feat, ".vscode/settings.json"), "{}")
	rc, out := s.run(t, "--dry-run")
	if rc != 0 || !strings.Contains(out, "update .vscode/settings.json in main, feat2") {
		t.Fatalf("rc=%d\n%s", rc, out)
	}
	for _, p := range []string{filepath.Join(s.main, ".vscode/settings.json"), s.manifest, filepath.Join(s.f.Root, ".git", "info", "devex", "sync-backups")} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s written by --dry-run", p)
		}
	}
}

func TestSyncBlockedByMarkersAndNoCopyPaths(t *testing.T) {
	s := newSyncFix(t)
	write(t, filepath.Join(s.main, ".vscode/settings.json.devex-conflict-feat"), "x")
	rc, out := s.run(t)
	if rc != 1 || !strings.Contains(out, "Sync blocked: Unresolved conflicts detected.") || !strings.Contains(out, "  - main/.vscode/settings.json.devex-conflict-feat (at: ") {
		t.Fatalf("rc=%d\n%s", rc, out)
	}
	write(t, filepath.Join(s.f.Root, ".devex.conf"), "[copies]\npaths =\n")
	if rc, out = s.run(t); rc != 0 || !strings.Contains(out, "Nothing to sync.") {
		t.Fatalf("rc=%d\n%s", rc, out)
	}
}
