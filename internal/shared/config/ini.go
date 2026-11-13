// Package config loads .devex.conf files (repo-level and global skills).
package config

import (
	"bufio"
	"errors"
	"os"
	"strings"
)

// readINI parses sections/keys like the Bash readers: trims whitespace, skips
// blank and '#' lines. A missing file yields an empty map and no error.
func readINI(path string) (map[string]map[string]string, error) {
	out := map[string]map[string]string{}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	section := ""
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = line[1 : len(line)-1]
			continue
		}
		if k, v, ok := strings.Cut(line, "="); ok {
			if out[section] == nil {
				out[section] = map[string]string{}
			}
			out[section][strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return out, sc.Err()
}

// splitList splits on commas, trimming and dropping empties.
func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
