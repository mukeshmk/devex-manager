// Package skills syncs AI-agent skill folders against a master directory (dx-skills-tools/).
package skills

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mukeshmk/devex-manager/internal/config"
)

type Copy struct {
	Location string // "master" or tool name
	Dir      string
	Mtime    int64
}

type Skill struct {
	Name        string
	Copies      []Copy
	Newest      string // location of the newest copy, "" if none
	Description string
}

func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// roots lists master then each active tool, in config order.
func roots(cfg config.SkillsConfig) []Copy {
	r := []Copy{{Location: "master", Dir: cfg.MasterDir}}
	for _, t := range cfg.Tools {
		r = append(r, Copy{Location: t.Name, Dir: t.Dir})
	}
	return r
}

// Locations returns every existing <root>/<name> dir, master first (dx-skills-diff:15-32).
func Locations(cfg config.SkillsConfig, name string) []Copy {
	var out []Copy
	for _, r := range roots(cfg) {
		d := filepath.Join(r.Dir, name)
		if isDir(d) {
			out = append(out, Copy{Location: r.Location, Dir: d, Mtime: dirMtime(d)})
		}
	}
	return out
}

// dirMtime is the newest regular-file mtime under dir, else dir's own (get_dir_mtime).
func dirMtime(dir string) int64 {
	var m int64
	filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() {
			if info, err := d.Info(); err == nil {
				m = max(m, info.ModTime().Unix())
			}
		}
		return nil
	})
	if m == 0 {
		if st, err := os.Stat(dir); err == nil {
			m = st.ModTime().Unix()
		}
	}
	return m
}

// SkillFile returns dir/SKILL.md, else dir/skill.md, else "" (get_skill_file).
func SkillFile(dir string) string {
	for _, n := range []string{"SKILL.md", "skill.md"} {
		if st, err := os.Stat(filepath.Join(dir, n)); err == nil && st.Mode().IsRegular() {
			return filepath.Join(dir, n)
		}
	}
	return ""
}

// description mirrors parse_frontmatter_field: first description: line between the first two ---.
func description(file string) string {
	f, err := os.Open(file)
	if err != nil {
		return ""
	}
	defer f.Close()
	count := 0
	for sc := bufio.NewScanner(f); sc.Scan(); {
		l := sc.Text()
		if l == "---" {
			if count++; count > 1 {
				break
			}
			continue
		}
		if v, ok := strings.CutPrefix(l, "description:"); count == 1 && ok {
			v = strings.TrimLeft(v, " \t")
			if v != "" && strings.ContainsRune(`"'`, rune(v[0])) { // one quote each end, like the awk gsub
				v = v[1:]
			}
			if n := len(v); n > 0 && strings.ContainsRune(`"'`, rune(v[n-1])) {
				v = v[:n-1]
			}
			return v
		}
	}
	return ""
}

// Scan lists every non-ignored skill (a child dir with SKILL.md or skill.md) across master and tools.
func Scan(cfg config.SkillsConfig) ([]Skill, error) {
	var names []string
	for _, r := range roots(cfg) {
		ents, err := os.ReadDir(r.Dir)
		if err != nil {
			continue // missing master dir: like `[ -d "$MASTER_DIR" ]`
		}
		for _, e := range ents {
			n := e.Name()
			if n == ".backups" || n == "backups" || slices.Contains(names, n) || cfg.IsIgnored(n) {
				continue
			}
			if d := filepath.Join(r.Dir, n); isDir(d) && SkillFile(d) != "" {
				names = append(names, n)
			}
		}
	}
	slices.Sort(names)
	out := make([]Skill, 0, len(names))
	for _, n := range names {
		s := Skill{Name: n, Copies: Locations(cfg, n), Description: "No description provided"}
		var best int64
		newestDir := ""
		for _, c := range s.Copies {
			if c.Mtime > best { // strict: ties keep the earlier location
				best, s.Newest, newestDir = c.Mtime, c.Location, c.Dir
			}
		}
		if newestDir != "" {
			if d := description(SkillFile(newestDir)); d != "" {
				s.Description = d
			}
		}
		out = append(out, s)
	}
	return out, nil
}
