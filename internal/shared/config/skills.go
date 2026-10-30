package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type SkillTool struct {
	Name, Dir string
	SyncOnly  []string
}

type SkillsConfig struct {
	MasterDir        string
	Tools            []SkillTool
	SyncOnly, Ignore []string
	Warning          string
}

var defaultToolDirs = map[string]string{
	"claude": ".claude/skills",
	"kiro":   ".kiro/skills",
	"gemini": ".gemini/antigravity/skills",
}

// LoadSkills resolves env > ~/.devex.conf [skills] > defaults.
func LoadSkills(home string, getenv func(string) string) (SkillsConfig, error) {
	confPath := filepath.Join(home, ".devex.conf")
	ini, err := readINI(confPath)
	if err != nil {
		return SkillsConfig{}, err
	}
	sk := ini["skills"]
	// first non-empty of env, file value, default (Bash -n/-z semantics)
	pick := func(env, key, def string) string {
		if v := getenv(env); v != "" {
			return v
		}
		if v := sk[key]; v != "" {
			return v
		}
		return def
	}
	expand := func(p string) string {
		if p == "~" || strings.HasPrefix(p, "~/") {
			return home + p[1:]
		}
		return p
	}

	var c SkillsConfig
	c.Warning = skillsWarning(confPath)
	c.MasterDir = strings.TrimSuffix(expand(pick("DX_MASTER_SKILLS_DIR", "master_dir", filepath.Join(home, "personal/skills"))), "/")

	tools := strings.FieldsFunc(pick("DX_TOOLS", "tools", "claude,kiro,gemini"),
		func(r rune) bool { return r == ',' || r == ' ' || r == '\t' || r == '\n' })
	for _, t := range tools {
		up := strings.ToUpper(t)
		dir := getenv("DX_" + up + "_SKILLS_DIR")
		if dir == "" {
			dir = getenv("DX_" + up + "_DIR")
		}
		if dir == "" {
			dir = sk[t+"_dir"]
		}
		if dir == "" && defaultToolDirs[t] != "" {
			dir = filepath.Join(home, defaultToolDirs[t])
		}
		if dir == "" {
			continue
		}
		dir = expand(dir)
		if st, err := os.Stat(dir); err != nil || !st.IsDir() {
			continue
		}
		c.Tools = append(c.Tools, SkillTool{Name: t, Dir: dir,
			SyncOnly: splitList(pick("DX_"+up+"_SYNC_ONLY", t+"_sync_only", ""))})
	}
	c.SyncOnly = splitList(pick("DX_SYNC_ONLY", "sync_only", ""))
	c.Ignore = splitList(pick("DX_IGNORE", "ignore", ""))
	return c, nil
}

func skillsWarning(confPath string) string {
	b, err := os.ReadFile(confPath)
	if err != nil {
		return "Warning: Global configuration file ~/.devex.conf not found.\n" +
			"Please configure your master skills path and tool directories in ~/.devex.conf."
	}
	s := string(b)
	if strings.Contains(s, "<path_to_master_skills_dir>") || strings.Contains(s, "<tool_1>") {
		return "Warning: ~/.devex.conf has not been customized (currently using template defaults).\n" +
			"Please check and update your master skills path and tool directories in ~/.devex.conf."
	}
	return ""
}

// IsIgnored mirrors is_ignored: excluded by a non-empty sync_only allowlist or listed in ignore.
func (c SkillsConfig) IsIgnored(name string) bool {
	if len(c.SyncOnly) > 0 && !slices.Contains(c.SyncOnly, name) {
		return true
	}
	return slices.Contains(c.Ignore, name)
}
