package skills

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/mukeshmk/devex-manager/internal/config"
)

// Remove backs up every copy to <root>/deleted/<location>/<name>, then deletes them all
// (dx-skills-rm). Nothing is deleted unless every backup succeeded.
func Remove(cfg config.SkillsConfig, name string, now time.Time) (string, error) {
	locs := Locations(cfg, name)
	if len(locs) == 0 {
		return "", fmt.Errorf("Error: Skill '%s' not found.", name)
	}
	b := newBackupRoot(cfg.MasterDir, now)
	for _, c := range locs {
		if err := b.save(c.Dir, "deleted", c.Location, name); err != nil {
			return b.root, fmt.Errorf("Error: backup of %s failed, nothing deleted: %w", c.Dir, err)
		}
	}
	for _, c := range locs {
		if err := os.RemoveAll(c.Dir); err != nil {
			return b.root, fmt.Errorf("Error: deleting %s: %w", c.Dir, err)
		}
	}
	return b.root, nil
}

// Diff returns `diff -ruN a b`; exit 1 only means the dirs differ.
func Diff(dirA, dirB string) (string, error) {
	out, err := exec.Command("diff", "-ruN", dirA, dirB).Output()
	if ee := (*exec.ExitError)(nil); errors.As(err, &ee) && ee.ExitCode() == 1 {
		err = nil
	}
	return string(out), err
}

// CreateTemplate writes <dir>/<name>/SKILL.md with the dx-skills-edit template.
func CreateTemplate(dir, name string) error {
	d := filepath.Join(dir, name)
	if err := os.MkdirAll(d, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(d, "SKILL.md"), []byte(fmt.Sprintf(
		"---\nname: %[1]s\ndescription: Description for %[1]s\n---\n\n# %[1]s\n\nInstructions and guidelines for %[1]s.\n", name)), 0o644)
}
