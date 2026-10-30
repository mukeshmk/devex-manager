package wtsync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestApplyBacksUpBeforeOverwrite(t *testing.T) {
	tmp := t.TempDir()
	main, feat := filepath.Join(tmp, "main"), filepath.Join(tmp, "feat")
	put(t, filepath.Join(feat, settings), "new")
	put(t, filepath.Join(main, settings), "old")
	mt := time.Unix(1700000000, 0)
	os.Chtimes(filepath.Join(feat, settings), mt, mt)
	os.Chmod(filepath.Join(feat, settings), 0o600)
	backup := filepath.Join(tmp, "bk")
	err := Apply([]Op{cp(feat, main, settings)}, backup)
	if err != nil {
		t.Fatal(err)
	}
	if got := read(t, filepath.Join(backup, "main", settings)); got != "old" {
		t.Fatalf("backup = %q", got)
	}
	dst := filepath.Join(main, settings)
	info, _ := os.Stat(dst)
	if read(t, dst) != "new" || !info.ModTime().Equal(mt) || info.Mode().Perm() != 0o600 {
		t.Fatalf("dst %q mtime %v mode %v", read(t, dst), info.ModTime(), info.Mode())
	}
	// a new file needs no backup, and none is created
	b2 := filepath.Join(tmp, "bk2")
	if err := Apply([]Op{cp(feat, filepath.Join(tmp, "feat2"), settings)}, b2); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(b2); !os.IsNotExist(err) {
		t.Fatalf("backup dir created without need: %v", err)
	}
	// delete backs up too
	if err := Apply([]Op{{Kind: Delete, Dst: Loc{main, settings}}}, b2); err != nil {
		t.Fatal(err)
	}
	if read(t, filepath.Join(b2, "main", settings)) != "new" {
		t.Fatal("delete not backed up")
	}
}

func TestApplyStopsOnError(t *testing.T) {
	tmp := t.TempDir()
	main, feat, feat2, feat3 := filepath.Join(tmp, "main"), filepath.Join(tmp, "feat"), filepath.Join(tmp, "feat2"), filepath.Join(tmp, "feat3")
	put(t, filepath.Join(main, settings), "v")
	put(t, filepath.Join(feat2, settings), "old2")
	ro := filepath.Join(feat2, ".vscode")
	os.Chmod(ro, 0o555)
	t.Cleanup(func() { os.Chmod(ro, 0o755) })
	backup := filepath.Join(tmp, "bk")
	err := Apply([]Op{cp(main, feat, settings), cp(main, feat2, settings), cp(main, feat3, settings)}, backup)
	if err == nil || !strings.Contains(err.Error(), backup) {
		t.Fatalf("err = %v, want mention of %s", err, backup)
	}
	if read(t, filepath.Join(feat, settings)) != "v" {
		t.Fatal("first op not applied")
	}
	if read(t, filepath.Join(feat2, settings)) != "old2" {
		t.Fatal("failed dst changed")
	}
	if _, err := os.Stat(filepath.Join(feat3, settings)); !os.IsNotExist(err) {
		t.Fatal("third op applied after error")
	}
	if ents, _ := os.ReadDir(ro); len(ents) != 1 {
		t.Fatalf("temp file left behind: %v", ents)
	}
}

func TestApplyMissingSourceIsError(t *testing.T) {
	tmp := t.TempDir()
	main, feat := filepath.Join(tmp, "main"), filepath.Join(tmp, "feat")
	put(t, filepath.Join(feat, settings), "keep")
	err := Apply([]Op{cp(main, feat, settings)}, filepath.Join(tmp, "bk"))
	if err == nil {
		t.Fatal("copy from a missing source must fail")
	}
	if read(t, filepath.Join(feat, settings)) != "keep" {
		t.Fatal("dst touched by a copy with a missing source")
	}
}

// scripted answers Choose/Confirm in order and fails the test on an extra question.
type scripted struct {
	t       *testing.T
	answers []int
}

func (s *scripted) next(q string) int {
	s.t.Helper()
	if len(s.answers) == 0 {
		s.t.Fatalf("unexpected question %q", q)
	}
	a := s.answers[0]
	s.answers = s.answers[1:]
	return a
}
func (s *scripted) Confirm(q string, _ bool) bool                     { return s.next(q) == 0 }
func (s *scripted) Choose(q string, _ []string, _ int) int            { return s.next(q) }
func (s *scripted) MultiSelect(q string, _ []string, _ []bool) []bool { s.next(q); return nil }

func TestResolveR4RechecksSameAfterEarlierAnswer(t *testing.T) {
	tmp := t.TempDir()
	main, feat, feat2 := filepath.Join(tmp, "main"), filepath.Join(tmp, "feat"), filepath.Join(tmp, "feat2")
	put(t, filepath.Join(main, settings), "M")
	put(t, filepath.Join(feat, settings), "A")
	put(t, filepath.Join(feat2, settings), "B")
	files := []FileState{fstate(settings, s(100, 1), s(200, 1), map[string]Stat{feat: s(200, 1), feat2: s(200, 1)})}
	p := BuildPlan(main, []string{feat, feat2}, files, SameFile)
	if len(p.Steps) != 2 {
		t.Fatalf("want two R4 conflicts, got %+v", p.Steps)
	}
	// Resolve Later on the first still asks about the second
	pr := &scripted{t: t, answers: []int{2, 2}}
	Resolve(p, pr)
	if len(pr.answers) != 0 {
		t.Fatal("second R4 conflict not asked after Resolve Later")
	}
	pr = &scripted{t: t, answers: []int{1}} // Keep feat; feat2 is then identical and must not be asked
	ops := Resolve(p, pr)
	if err := Apply(ops, filepath.Join(tmp, "bk")); err != nil {
		t.Fatal(err)
	}
	for _, w := range []string{main, feat, feat2} {
		if got := read(t, filepath.Join(w, settings)); got != "A" {
			t.Fatalf("%s = %q, want A", w, got)
		}
	}
	if _, err := os.Stat(filepath.Join(main, settings+conflictTag+"feat2")); !os.IsNotExist(err) {
		t.Fatal("conflict file written for an aligned worktree")
	}
}

func TestApplyBacksUpThirdWorktreeOnKeepA(t *testing.T) {
	tmp := t.TempDir()
	main, feat, feat2, feat3 := filepath.Join(tmp, "main"), filepath.Join(tmp, "feat"), filepath.Join(tmp, "feat2"), filepath.Join(tmp, "feat3")
	put(t, filepath.Join(main, settings), "M")
	put(t, filepath.Join(feat, settings), "A")
	put(t, filepath.Join(feat2, settings), "B")
	put(t, filepath.Join(feat3, settings), "C")
	files := []FileState{fstate(settings, s(100, 1), s(100, 1), map[string]Stat{feat: s(200, 1), feat2: s(200, 1), feat3: s(200, 1)})}
	p := BuildPlan(main, []string{feat, feat2, feat3}, files, SameFile)
	backup := filepath.Join(tmp, "bk")
	if err := Apply(Resolve(p, &scripted{t: t, answers: []int{0}}), backup); err != nil {
		t.Fatal(err)
	}
	if read(t, filepath.Join(feat3, settings)) != "A" {
		t.Fatal("Keep feat did not fan out")
	}
	for w, old := range map[string]string{"main": "M", "feat2": "B", "feat3": "C"} {
		if got := read(t, filepath.Join(backup, w, settings)); got != old {
			t.Fatalf("backup of %s = %q, want %q", w, got, old)
		}
	}
}

func TestPruneEmptyDirs(t *testing.T) {
	tmp := t.TempDir()
	put(t, filepath.Join(tmp, "w", ".vscode", "keep"), "x")
	os.MkdirAll(filepath.Join(tmp, "w", ".vscode", "a", "b"), 0o755)
	os.MkdirAll(filepath.Join(tmp, "w", "other", "empty"), 0o755)
	PruneEmptyDirs([]string{filepath.Join(tmp, "w")}, []string{".vscode", ".env"})
	if _, err := os.Stat(filepath.Join(tmp, "w", ".vscode", "a")); !os.IsNotExist(err) {
		t.Fatal("empty nested dirs not pruned")
	}
	if _, err := os.Stat(filepath.Join(tmp, "w", "other", "empty")); err != nil {
		t.Fatal("pruned outside copy paths")
	}
}
