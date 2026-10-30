// Package nb inspects and discovers Jupyter notebooks.
package nb

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Info struct {
	Path                  string
	Size                  int64
	Cells, Code, Markdown int
	HasOutput             bool
	Modified              time.Time
	Err                   error
}

func Inspect(path string) Info {
	i := Info{Path: path}
	st, err := os.Stat(path)
	if err != nil {
		i.Err = err
		return i
	}
	i.Size, i.Modified = st.Size(), st.ModTime()
	b, err := os.ReadFile(path)
	if err != nil {
		i.Err = err
		return i
	}
	var doc struct {
		Cells []struct {
			Type    string            `json:"cell_type"`
			Outputs []json.RawMessage `json:"outputs"`
		} `json:"cells"`
	}
	if i.Err = json.Unmarshal(b, &doc); i.Err != nil {
		return i
	}
	i.Cells = len(doc.Cells)
	for _, c := range doc.Cells {
		switch c.Type {
		case "code":
			i.Code++
		case "markdown":
			i.Markdown++
		}
		if len(c.Outputs) > 0 {
			i.HasOutput = true
		}
	}
	return i
}

// Find returns *.ipynb paths under root (relative to root), skipping dot-prefixed segments.
func Find(root string) []string {
	var out []string
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if p != root && strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".ipynb") {
			rel, _ := filepath.Rel(root, p)
			out = append(out, rel)
		}
		return nil
	})
	return out
}
