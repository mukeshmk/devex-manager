package skills

import (
	"os/exec"
	"path/filepath"
	"slices"

	"github.com/mukeshmk/devex-manager/internal/shared/config"
)

type Update struct{ Skill, From, FromDir, To, ToDir string }

// SameDir is `diff -rq a b` succeeding (the sync check in dx-skills-sync:44,62).
func SameDir(a, b string) bool { return exec.Command("diff", "-rq", a, b).Run() == nil }

// Plan lists the copies that differ from the newest one (dx-skills-sync:11-67).
func Plan(cfg config.SkillsConfig, skills []Skill, same func(a, b string) bool) []Update {
	var ups []Update
	for _, s := range skills {
		if s.Newest == "" {
			continue
		}
		var from string
		for _, c := range s.Copies {
			if c.Location == s.Newest {
				from = c.Dir
			}
		}
		add := func(to, root string) {
			dest := filepath.Join(root, s.Name)
			if to != s.Newest && (!isDir(dest) || !same(from, dest)) {
				ups = append(ups, Update{s.Name, s.Newest, from, to, dest})
			}
		}
		add("master", cfg.MasterDir)
		for _, t := range cfg.Tools {
			if len(t.SyncOnly) == 0 || slices.Contains(t.SyncOnly, s.Name) {
				add(t.Name, t.Dir)
			}
		}
	}
	return ups
}
