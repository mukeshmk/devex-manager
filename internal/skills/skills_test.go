package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mukeshmk/devex-manager/internal/shared/config"
)

func write(t *testing.T, path, content string, mt time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if !mt.IsZero() {
		os.Chtimes(path, mt, mt)
	}
}

func fixture(t *testing.T) config.SkillsConfig {
	root := t.TempDir()
	cfg := config.SkillsConfig{MasterDir: filepath.Join(root, "master")}
	for _, n := range []string{"claude", "kiro"} {
		d := filepath.Join(root, n)
		os.MkdirAll(d, 0o755)
		cfg.Tools = append(cfg.Tools, config.SkillTool{Name: n, Dir: d})
	}
	os.MkdirAll(cfg.MasterDir, 0o755)
	return cfg
}

var t0 = time.Date(2026, 5, 27, 21, 0, 0, 0, time.Local)

func TestSkillsScan(t *testing.T) {
	cfg := fixture(t)
	write(t, filepath.Join(cfg.Tools[0].Dir, "a/skill.md"), "---\nname: a\ndescription: \"Old one\"\n---\n", t0)
	write(t, filepath.Join(cfg.Tools[1].Dir, "a/SKILL.md"), "---\ndescription: New one\n---\n", t0.Add(time.Hour))
	// tie between master and claude: master wins
	write(t, filepath.Join(cfg.MasterDir, "b/SKILL.md"), "no frontmatter", t0)
	write(t, filepath.Join(cfg.Tools[0].Dir, "b/SKILL.md"), "no frontmatter", t0)
	write(t, filepath.Join(cfg.MasterDir, ".backups/x/SKILL.md"), "", t0)
	write(t, filepath.Join(cfg.MasterDir, "notaskill/readme"), "", t0)
	write(t, filepath.Join(cfg.MasterDir, "ign/SKILL.md"), "", t0)
	cfg.Ignore = []string{"ign"}
	got, err := Scan(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name != "a" || got[1].Name != "b" {
		t.Fatalf("got %+v", got)
	}
	if got[0].Newest != "kiro" || got[0].Description != "New one" || len(got[0].Copies) != 2 {
		t.Errorf("a = %+v", got[0])
	}
	if got[0].Copies[1].Mtime != t0.Add(time.Hour).Unix() {
		t.Errorf("mtime %d", got[0].Copies[1].Mtime)
	}
	if got[1].Newest != "master" || got[1].Description != "No description provided" {
		t.Errorf("b = %+v", got[1])
	}
}

func TestSkillsPlanSyncOnly(t *testing.T) {
	cfg := fixture(t)
	cfg.Tools[1].SyncOnly = []string{"other"}
	write(t, filepath.Join(cfg.Tools[0].Dir, "a/SKILL.md"), "x", t0)
	sk, _ := Scan(cfg)
	ups := Plan(cfg, sk, SameDir)
	if len(ups) != 1 || ups[0].To != "master" || ups[0].From != "claude" ||
		ups[0].ToDir != filepath.Join(cfg.MasterDir, "a") || ups[0].FromDir != filepath.Join(cfg.Tools[0].Dir, "a") {
		t.Fatalf("ups = %+v", ups)
	}
	write(t, filepath.Join(cfg.MasterDir, "a/SKILL.md"), "x", t0)
	sk, _ = Scan(cfg)
	if ups := Plan(cfg, sk, SameDir); len(ups) != 0 {
		t.Fatalf("in sync, got %+v", ups)
	}
}

func TestSkillsApplyMirrorsDeletes(t *testing.T) {
	cfg := fixture(t)
	from, to := filepath.Join(cfg.Tools[0].Dir, "a"), filepath.Join(cfg.MasterDir, "a")
	write(t, filepath.Join(from, "SKILL.md"), "new", t0)
	write(t, filepath.Join(from, "sub/run.sh"), "#!/bin/sh", t0)
	os.Chmod(filepath.Join(from, "sub/run.sh"), 0o755)
	write(t, filepath.Join(to, "SKILL.md"), "old", time.Time{})
	write(t, filepath.Join(to, "extra.txt"), "gone", time.Time{})
	write(t, filepath.Join(to, "olddir/f"), "gone", time.Time{})
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.Local)
	root, _, err := ApplyUpdates([]Update{{Skill: "a", From: "claude", FromDir: from, To: "master", ToDir: to}}, cfg.MasterDir, now)
	if err != nil {
		t.Fatal(err)
	}
	if root != filepath.Join(cfg.MasterDir, ".backups", "2026-10-30-120000") {
		t.Fatalf("root %s", root)
	}
	if !SameDir(from, to) {
		t.Fatal("dest not mirrored")
	}
	for _, p := range []string{"extra.txt", "olddir"} {
		if _, err := os.Lstat(filepath.Join(to, p)); err == nil {
			t.Errorf("%s not deleted", p)
		}
	}
	st, _ := os.Stat(filepath.Join(to, "sub/run.sh"))
	if st.Mode().Perm() != 0o755 || !st.ModTime().Equal(t0) {
		t.Errorf("mode/mtime not kept: %v %v", st.Mode(), st.ModTime())
	}
	b, _ := os.ReadFile(filepath.Join(root, "master/a/extra.txt"))
	if string(b) != "gone" {
		t.Errorf("backup missing: %q", b)
	}
	// second run in the same second must not nest into the first backup
	write(t, filepath.Join(to, "SKILL.md"), "changed", time.Time{})
	root2, _, err := ApplyUpdates([]Update{{Skill: "a", From: "claude", FromDir: from, To: "master", ToDir: to}}, cfg.MasterDir, now)
	if err != nil || root2 != root+"-2" {
		t.Fatalf("root2 %s %v", root2, err)
	}
}

func TestSkillsApplyBackupFailureSkips(t *testing.T) {
	cfg := fixture(t)
	from, to := filepath.Join(cfg.Tools[0].Dir, "a"), filepath.Join(cfg.MasterDir, "a")
	write(t, filepath.Join(from, "SKILL.md"), "new", t0)
	write(t, filepath.Join(to, "SKILL.md"), "old", t0)
	write(t, filepath.Join(cfg.MasterDir, ".backups"), "a file blocks backups", t0)
	_, _, err := ApplyUpdates([]Update{{Skill: "a", From: "claude", FromDir: from, To: "master", ToDir: to}}, cfg.MasterDir, time.Now())
	if err == nil || !strings.HasPrefix(err.Error(), "Error: ") {
		t.Fatalf("want error, got %v", err)
	}
	b, _ := os.ReadFile(filepath.Join(to, "SKILL.md"))
	if string(b) != "old" {
		t.Fatal("dest changed without a backup")
	}
}

func TestSkillsRemove(t *testing.T) {
	cfg := fixture(t)
	write(t, filepath.Join(cfg.MasterDir, "a/SKILL.md"), "m", t0)
	write(t, filepath.Join(cfg.Tools[1].Dir, "a/SKILL.md"), "k", t0)
	root, err := Remove(cfg, "a", time.Date(2026, 10, 30, 12, 0, 0, 0, time.Local))
	if err != nil {
		t.Fatal(err)
	}
	if root != filepath.Join(cfg.MasterDir, ".backups", "2026-10-30-120000") {
		t.Fatalf("root %s", root)
	}
	for _, loc := range []string{"master", "kiro"} {
		if _, err := os.Stat(filepath.Join(root, "deleted", loc, "a/SKILL.md")); err != nil {
			t.Errorf("backup %s: %v", loc, err)
		}
	}
	if _, err := os.Stat(filepath.Join(cfg.MasterDir, "a")); err == nil {
		t.Error("master copy not deleted")
	}
	if _, err := Remove(cfg, "a", time.Now()); err == nil || err.Error() != "Error: Skill 'a' not found." {
		t.Errorf("missing skill err = %v", err)
	}
}

func TestSkillsDiffAndTemplate(t *testing.T) {
	cfg := fixture(t)
	if err := CreateTemplate(cfg.MasterDir, "n"); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(cfg.MasterDir, "n/SKILL.md"))
	want := "---\nname: n\ndescription: Description for n\n---\n\n# n\n\nInstructions and guidelines for n.\n"
	if string(b) != want {
		t.Fatalf("template %q", b)
	}
	other := filepath.Join(cfg.Tools[0].Dir, "n")
	write(t, filepath.Join(other, "SKILL.md"), strings.Replace(want, "Instructions", "Changed", 1), t0)
	out, err := Diff(filepath.Join(cfg.MasterDir, "n"), other)
	if err != nil || !strings.Contains(out, "-Instructions and guidelines for n.") || !strings.Contains(out, "+Changed") {
		t.Fatalf("diff %q %v", out, err)
	}
	if out, err := Diff(other, other); err != nil || out != "" {
		t.Fatalf("same diff %q %v", out, err)
	}
}

func TestSkillsInvalidNames(t *testing.T) {
	cfg := fixture(t)
	write(t, filepath.Join(cfg.MasterDir, "a/SKILL.md"), "m", t0)
	for _, n := range []string{"", ".", "..", "a/b", "../x", ".backups", "backups"} {
		if l := Locations(cfg, n); l != nil {
			t.Errorf("Locations(%q) = %v", n, l)
		}
		if _, err := Remove(cfg, n, time.Now()); err == nil {
			t.Errorf("Remove(%q) succeeded", n)
		}
		if err := CreateTemplate(cfg.MasterDir, n); err == nil {
			t.Errorf("CreateTemplate(%q) succeeded", n)
		}
	}
	if _, err := os.Stat(filepath.Join(cfg.MasterDir, "a/SKILL.md")); err != nil {
		t.Fatal("master damaged")
	}
	if _, err := os.Stat(filepath.Join(cfg.MasterDir, ".backups")); err == nil {
		t.Fatal("backup written for an invalid name")
	}
}

func TestSkillsApplySymlinkedFrom(t *testing.T) {
	cfg := fixture(t)
	real := filepath.Join(t.TempDir(), "real")
	write(t, filepath.Join(real, "SKILL.md"), "new", t0)
	from, to := filepath.Join(cfg.Tools[0].Dir, "a"), filepath.Join(cfg.MasterDir, "a")
	os.Symlink(real, from)
	write(t, filepath.Join(to, "SKILL.md"), "old", t0)
	if _, _, err := ApplyUpdates([]Update{{"a", "claude", from, "master", to}}, cfg.MasterDir, time.Now()); err != nil {
		t.Fatal(err)
	}
	if st, err := os.Lstat(to); err != nil || !st.IsDir() {
		t.Fatalf("dest not a real dir: %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(to, "SKILL.md")); string(b) != "new" {
		t.Fatalf("dest %q", b)
	}
}

func TestSkillsApplySymlinkedTo(t *testing.T) {
	cfg := fixture(t)
	from := filepath.Join(cfg.MasterDir, "a")
	write(t, filepath.Join(from, "SKILL.md"), "new", t0)
	real := filepath.Join(t.TempDir(), "real")
	write(t, filepath.Join(real, "SKILL.md"), "old", t0)
	write(t, filepath.Join(real, "extra"), "x", t0)
	to := filepath.Join(cfg.Tools[0].Dir, "a")
	os.Symlink(real, to)
	root, _, err := ApplyUpdates([]Update{{"a", "master", from, "claude", to}}, cfg.MasterDir, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	bk := filepath.Join(root, "claude/a")
	if st, err := os.Lstat(bk); err != nil || !st.IsDir() {
		t.Fatalf("backup not a real dir: %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(bk, "SKILL.md")); string(b) != "old" {
		t.Fatalf("backup content %q", b)
	}
	// mirrored through the link into its target, like rsync "src/" "dst/"
	if st, _ := os.Lstat(to); st.Mode()&os.ModeSymlink == 0 {
		t.Fatal("dest symlink replaced")
	}
	if b, _ := os.ReadFile(filepath.Join(real, "SKILL.md")); string(b) != "new" {
		t.Fatalf("target %q", b)
	}
	if _, err := os.Stat(filepath.Join(real, "extra")); err == nil {
		t.Fatal("extra not deleted in target")
	}
}

func TestSkillsApplyReportsDone(t *testing.T) {
	cfg := fixture(t)
	from := filepath.Join(cfg.MasterDir, "a")
	write(t, filepath.Join(from, "SKILL.md"), "new", t0)
	write(t, filepath.Join(cfg.Tools[1].Dir, "blocker"), "file", t0)
	ups := []Update{{"a", "master", from, "claude", filepath.Join(cfg.Tools[0].Dir, "a")},
		{"a", "master", from, "kiro", filepath.Join(cfg.Tools[1].Dir, "blocker/a")}}
	_, done, err := ApplyUpdates(ups, cfg.MasterDir, time.Now())
	if err == nil || done != 1 {
		t.Fatalf("done=%d err=%v", done, err)
	}
}

func TestSkillsApplyRefusesBroadSymlinkTarget(t *testing.T) {
	home := t.TempDir()
	cfg := config.SkillsConfig{MasterDir: filepath.Join(home, "master")}
	claude := filepath.Join(home, "claude")
	cfg.Tools = []config.SkillTool{{Name: "claude", Dir: claude}}
	from := filepath.Join(cfg.MasterDir, "a")
	write(t, filepath.Join(from, "SKILL.md"), "new", t0)
	write(t, filepath.Join(home, "precious.txt"), "keep", t0)
	os.MkdirAll(claude, 0o755)
	to := filepath.Join(claude, "a")
	for _, target := range []string{home, claude, cfg.MasterDir, "/"} {
		os.Remove(to)
		os.Symlink(target, to)
		root, done, err := ApplyUpdates([]Update{{"a", "master", from, "claude", to}}, cfg.MasterDir, time.Now())
		if err == nil || done != 0 || root != "" {
			t.Fatalf("%s: root=%q done=%d err=%v", target, root, done, err)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(home, "precious.txt")); string(b) != "keep" {
		t.Fatal("home content deleted")
	}
	if _, err := os.Stat(filepath.Join(cfg.MasterDir, ".backups")); err == nil {
		t.Fatal("backup written")
	}
}
