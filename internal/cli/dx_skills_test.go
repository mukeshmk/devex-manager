package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type skillsFix struct{ home string }

// newSkillsFix mirrors tests/test_dx_skills.sh: env-configured master + gemini, claude, kiro.
func newSkillsFix(t *testing.T, extra map[string]string) *skillsFix {
	home := t.TempDir()
	env := map[string]string{
		"DX_MASTER_SKILLS_DIR": filepath.Join(home, "master"),
		"DX_GEMINI_SKILLS_DIR": filepath.Join(home, "gemini"),
		"DX_CLAUDE_SKILLS_DIR": filepath.Join(home, "claude"),
		"DX_KIRO_SKILLS_DIR":   filepath.Join(home, "kiro"),
		"DX_TOOLS":             "gemini,claude,kiro",
		"EDITOR":               "true", // never launch a real editor
	}
	for k, v := range extra {
		env[k] = v
	}
	for _, d := range []string{"master", "gemini", "claude", "kiro"} {
		os.MkdirAll(filepath.Join(home, d), 0o755)
	}
	write(t, filepath.Join(home, ".devex.conf"), "[skills]\nmaster_dir = ~/personal/skills\n")
	oldEnv, oldHome := skillsEnv, skillsHome
	skillsEnv = func(k string) string { return env[k] }
	skillsHome = func() (string, error) { return home, nil }
	t.Cleanup(func() { skillsEnv, skillsHome = oldEnv, oldHome })
	t.Chdir(home)
	return &skillsFix{home}
}

func (f *skillsFix) run(t *testing.T, stdin string, args ...string) (int, string, string) {
	t.Helper()
	in, err := os.CreateTemp(t.TempDir(), "in")
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	in.WriteString(stdin)
	in.Seek(0, 0)
	var out, errw bytes.Buffer
	rc := Run("dx", append([]string{"skills"}, args...), in, &out, &errw)
	return rc, out.String(), errw.String()
}

func (f *skillsFix) skill(t *testing.T, loc, name, body string, mt time.Time) {
	p := filepath.Join(f.home, loc, name, "skill.md")
	write(t, p, body)
	os.Chtimes(p, mt, mt)
}

const scrum = "---\nname: scrum-update\ndescription: Convert Zoom summaries to Obsidian notes\n---\n# Scrum Update\n"

var skT0 = time.Date(2026, 5, 27, 21, 0, 0, 0, time.Local)

func TestSkillsWarning(t *testing.T) {
	f := newSkillsFix(t, nil)
	if _, _, e := f.run(t, "", "list"); e != "" {
		t.Errorf("customized config warned: %q", e)
	}
	os.Remove(filepath.Join(f.home, ".devex.conf"))
	_, _, e := f.run(t, "", "list")
	if strings.Count(e, "Warning: Global configuration file ~/.devex.conf not found.") != 1 {
		t.Errorf("missing-config warning: %q", e)
	}
	write(t, filepath.Join(f.home, ".devex.conf"), "[skills]\ntools = <tool_1>\n")
	if _, _, e := f.run(t, "", "list"); !strings.Contains(e, "has not been customized") {
		t.Errorf("template warning: %q", e)
	}
}

func TestSkillsListAndNoArgs(t *testing.T) {
	f := newSkillsFix(t, nil)
	f.skill(t, "claude", "scrum-update", scrum+"Claude original content\n", skT0)
	for _, args := range [][]string{{"list"}, nil} {
		rc, out, _ := f.run(t, "", args...)
		for _, w := range []string{"Master Directory: " + filepath.Join(f.home, "master"), "Active Agents:    gemini,claude,kiro",
			"scrum-update", "Convert Zoom summaries", "claude", "2026-05-27 21:00:00", "Last Modified"} {
			if rc != 0 || !strings.Contains(out, w) {
				t.Errorf("%v rc=%d missing %q:\n%s", args, rc, w, out)
			}
		}
	}
	rc, _, e := f.run(t, "", "bogus")
	if rc != 1 || !strings.Contains(e, "Error: Unknown command 'bogus'") {
		t.Errorf("bogus rc=%d %q", rc, e)
	}
}

func TestSkillsSyncFlow(t *testing.T) {
	f := newSkillsFix(t, nil)
	f.skill(t, "claude", "scrum-update", scrum+"Claude original content\n", skT0)
	rc, out, e := f.run(t, "y\n", "sync")
	if rc != 0 || !strings.Contains(out, "[scrum-update]  Copying from claude  -->  master,gemini,kiro") ||
		!strings.Contains(out, "✓ Synchronization complete!") {
		t.Fatalf("rc=%d\n%s%s", rc, out, e)
	}
	for _, loc := range []string{"master", "gemini", "kiro"} {
		fileIs(t, filepath.Join(f.home, loc, "scrum-update/skill.md"), scrum+"Claude original content\n")
	}
	if rc, out, _ := f.run(t, "", "sync"); rc != 0 || !strings.Contains(out, "All skills are already synchronized!") {
		t.Fatalf("second sync rc=%d %s", rc, out)
	}

	f.skill(t, "kiro", "scrum-update", scrum+"Kiro modified content\n", skT0.Add(time.Hour))
	rc, out, _ = f.run(t, "n\n", "sync")
	if rc != 1 || !strings.Contains(out, "Copying from kiro  -->  master,gemini,claude") || !strings.Contains(out, "Synchronization cancelled.") {
		t.Fatalf("preview rc=%d\n%s", rc, out)
	}
	if rc, out, _ := f.run(t, "", "sync", "--yes"); rc != 0 || !strings.Contains(out, "Backups stored in: ") {
		t.Fatalf("--yes rc=%d %s", rc, out)
	}
	for _, loc := range []string{"master", "gemini", "claude"} {
		fileIs(t, filepath.Join(f.home, loc, "scrum-update/skill.md"), scrum+"Kiro modified content\n")
	}
	m, _ := filepath.Glob(filepath.Join(f.home, "master/.backups/*/claude/scrum-update/skill.md"))
	if len(m) != 1 {
		t.Fatalf("backups %v", m)
	}
	fileIs(t, m[0], scrum+"Claude original content\n")
}

func TestSkillsSyncEOFCancels(t *testing.T) {
	f := newSkillsFix(t, nil)
	f.skill(t, "claude", "a", scrum, skT0)
	rc, out, _ := f.run(t, "", "sync")
	if rc != 1 || !strings.Contains(out, "Synchronization cancelled.") {
		t.Fatalf("rc=%d %s", rc, out)
	}
	if _, err := os.Stat(filepath.Join(f.home, "master/a")); err == nil {
		t.Fatal("synced on EOF")
	}
}

func TestSkillsSyncOnlyAndIgnore(t *testing.T) {
	f := newSkillsFix(t, map[string]string{"DX_GEMINI_SYNC_ONLY": "other", "DX_IGNORE": "skipme"})
	f.skill(t, "claude", "a", scrum, skT0)
	f.skill(t, "claude", "skipme", scrum, skT0)
	rc, out, _ := f.run(t, "y\n", "sync")
	if rc != 0 || !strings.Contains(out, "Copying from claude  -->  master,kiro") || strings.Contains(out, "skipme") {
		t.Fatalf("rc=%d\n%s", rc, out)
	}
	if _, err := os.Stat(filepath.Join(f.home, "gemini/a")); err == nil {
		t.Error("gemini sync_only ignored")
	}
	if _, err := os.Stat(filepath.Join(f.home, "master/skipme")); err == nil {
		t.Error("ignored skill synced")
	}
	if _, out, _ := f.run(t, "", "list"); strings.Contains(out, "skipme") {
		t.Errorf("ignored skill listed:\n%s", out)
	}
}

func TestSkillsDiffCmd(t *testing.T) {
	f := newSkillsFix(t, nil)
	f.skill(t, "master", "a", "one\n", skT0)
	rc, out, _ := f.run(t, "", "diff", "a")
	if rc != 0 || !strings.Contains(out, "Skill 'a' only exists in master (no other versions to diff against).") {
		t.Fatalf("rc=%d %s", rc, out)
	}
	f.skill(t, "kiro", "a", "two\n", skT0)
	rc, out, _ = f.run(t, "", "diff", "a")
	if rc != 0 || !strings.Contains(out, "=== Diff: master vs kiro ===") || !strings.Contains(out, "-one") || !strings.Contains(out, "+two") {
		t.Fatalf("rc=%d %s", rc, out)
	}
	f.skill(t, "kiro", "a", "one\n", skT0)
	if _, out, _ = f.run(t, "", "diff", "a"); !strings.Contains(out, "Skill 'a' is in sync across all active targets (master kiro).") {
		t.Fatalf("%s", out)
	}
	if rc, _, e := f.run(t, "", "diff", "nope"); rc != 1 || !strings.Contains(e, "Error: Skill 'nope' not found in any active directory.") {
		t.Fatalf("rc=%d %q", rc, e)
	}
}

func TestSkillsRmCmd(t *testing.T) {
	f := newSkillsFix(t, nil)
	f.skill(t, "master", "a", "m", skT0)
	f.skill(t, "claude", "a", "c", skT0)
	if rc, out, _ := f.run(t, "", "rm", "a"); rc != 0 || !strings.Contains(out, "Cancelled.") {
		t.Fatalf("EOF rm rc=%d %s", rc, out)
	}
	rc, out, _ := f.run(t, "y\n", "rm", "a")
	for _, w := range []string{"WARNING: You are about to delete skill 'a' from: master claude", "  [delete] a from master... Done",
		"  [delete] a from claude... Done", "✓ Skill 'a' deleted successfully.", "Backup stored in: "} {
		if rc != 0 || !strings.Contains(out, w) {
			t.Errorf("rc=%d missing %q:\n%s", rc, w, out)
		}
	}
	for _, loc := range []string{"master", "claude"} {
		if _, err := os.Stat(filepath.Join(f.home, loc, "a")); err == nil {
			t.Errorf("%s copy remains", loc)
		}
		if m, _ := filepath.Glob(filepath.Join(f.home, "master/.backups/*/deleted", loc, "a/skill.md")); len(m) != 1 {
			t.Errorf("no backup for %s", loc)
		}
	}
	if rc, _, e := f.run(t, "y\n", "rm", "a"); rc != 1 || !strings.Contains(e, "Error: Skill 'a' not found.") {
		t.Errorf("rc=%d %q", rc, e)
	}
}

func TestSkillsEditCreatesAndSyncs(t *testing.T) {
	ed := filepath.Join(t.TempDir(), "ed.sh")
	write(t, ed, "#!/bin/sh\necho \"Edited Content\" >> \"$1\"\n")
	os.Chmod(ed, 0o755)
	f := newSkillsFix(t, map[string]string{"EDITOR": ed})
	if rc, out, _ := f.run(t, "", "edit", "new-skill"); rc != 0 || !strings.Contains(out, "Cancelled.") {
		t.Fatalf("EOF edit rc=%d %s", rc, out)
	}
	rc, out, e := f.run(t, "y\ny\n", "edit", "new-skill")
	if rc != 0 || !strings.Contains(out, "✓ Created skill folder and default SKILL.md template.") || !strings.Contains(out, "Changes saved.") {
		t.Fatalf("rc=%d\n%s%s", rc, out, e)
	}
	for _, loc := range []string{"master", "gemini", "claude", "kiro"} {
		b, err := os.ReadFile(filepath.Join(f.home, loc, "new-skill/SKILL.md"))
		if err != nil || !strings.Contains(string(b), "Edited Content") {
			t.Errorf("%s: %q %v", loc, b, err)
		}
	}
}

func TestSkillsRejectsBadNames(t *testing.T) {
	f := newSkillsFix(t, nil)
	f.skill(t, "master", "a", "m", skT0)
	f.skill(t, "claude", "a", "c", skT0)
	for _, args := range [][]string{{"rm", ""}, {"rm", ".."}, {"rm", "."}, {"rm", "a/b"}, {"edit", ""}, {"edit", "../x"}, {"diff", ""}} {
		if rc, out, e := f.run(t, "y\ny\n", args...); rc != 1 {
			t.Errorf("%q rc=%d\n%s%s", args, rc, out, e)
		}
	}
	for _, loc := range []string{"master", "claude"} {
		if _, err := os.Stat(filepath.Join(f.home, loc, "a/skill.md")); err != nil {
			t.Errorf("%s damaged: %v", loc, err)
		}
	}
	for _, p := range []string{"master/.backups", "x", "master/a/b"} {
		if _, err := os.Stat(filepath.Join(f.home, p)); err == nil {
			t.Errorf("%s created", p)
		}
	}
}
