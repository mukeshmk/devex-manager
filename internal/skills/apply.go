package skills

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"time"
)

// backupRoot lazily creates <master>/.backups/<ts> (or <ts>-N if taken) so a second run in the
// same second never copies into an earlier backup.
type backupRoot struct{ base, root string }

func newBackupRoot(master string, now time.Time) *backupRoot {
	return &backupRoot{base: filepath.Join(master, ".backups", now.Format("2006-01-02-150405"))}
}

func (b *backupRoot) get() (string, error) {
	if b.root != "" {
		return b.root, nil
	}
	if err := os.MkdirAll(filepath.Dir(b.base), 0o755); err != nil {
		return "", err
	}
	for n := 1; ; n++ {
		r := b.base
		if n > 1 {
			r = fmt.Sprintf("%s-%d", b.base, n)
		}
		err := os.Mkdir(r, 0o755)
		if err == nil {
			b.root = r
			return r, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return "", err
		}
	}
}

// save copies dir to <root>/<rel...> (cp -Rp).
func (b *backupRoot) save(dir string, rel ...string) error {
	r, err := b.get()
	if err != nil {
		return err
	}
	return mirror(dir, filepath.Join(append([]string{r}, rel...)...))
}

// ApplyUpdates backs up each existing destination, then mirrors FromDir onto ToDir
// (rsync -a --delete). Stops at the first error; returns the backup root used ("" if none).
func ApplyUpdates(ups []Update, master string, now time.Time) (string, error) {
	b := newBackupRoot(master, now)
	for _, u := range ups {
		if isDir(u.ToDir) {
			if err := b.save(u.ToDir, u.To, u.Skill); err != nil {
				return b.root, fmt.Errorf("Error: backup of %s failed, %s not synced: %w", u.ToDir, u.Skill, err)
			}
		}
		if err := mirror(u.FromDir, u.ToDir); err != nil {
			return b.root, fmt.Errorf("Error: sync of %s from %s to %s failed: %w", u.Skill, u.From, u.To, err)
		}
	}
	return b.root, nil
}

// mirror makes dst an exact copy of src: files, symlinks, modes and mtimes; extras in dst are deleted.
func mirror(src, dst string) error {
	var dirs []string
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		// a different kind of entry in the way (dir vs file vs link) is replaced; the root is
		// left alone so a symlinked destination dir is followed, like rsync "src/" "dst/"
		if old, err := os.Lstat(target); rel != "." && err == nil && old.Mode().Type() != info.Mode().Type() {
			if err := os.RemoveAll(target); err != nil {
				return err
			}
		}
		switch {
		case d.IsDir():
			dirs = append(dirs, p)
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			return os.Chmod(target, info.Mode().Perm())
		case info.Mode()&fs.ModeSymlink != 0:
			link, err := os.Readlink(p)
			if err != nil {
				return err
			}
			os.Remove(target)
			return os.Symlink(link, target)
		case info.Mode().IsRegular():
			return copyFile(p, target, info)
		}
		return nil // sockets, devices: rsync -a without -D skips them too
	})
	if err != nil {
		return err
	}
	var extras []string
	filepath.WalkDir(dst, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(dst, p)
		if _, err := os.Lstat(filepath.Join(src, rel)); errors.Is(err, fs.ErrNotExist) {
			extras = append(extras, p)
			if d.IsDir() {
				return filepath.SkipDir
			}
		}
		return nil
	})
	for _, p := range extras {
		if err := os.RemoveAll(p); err != nil {
			return err
		}
	}
	slices.Reverse(dirs) // children first, so setting a parent's mtime is final
	for _, p := range dirs {
		rel, _ := filepath.Rel(src, p)
		if st, err := os.Stat(p); err == nil {
			os.Chtimes(filepath.Join(dst, rel), st.ModTime(), st.ModTime())
		}
	}
	return nil
}

func copyFile(src, dst string, info fs.FileInfo) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	os.Remove(dst) // a read-only old copy would block O_TRUNC; it is already backed up
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chmod(dst, info.Mode().Perm())
	}
	if err == nil {
		err = os.Chtimes(dst, info.ModTime(), info.ModTime())
	}
	return err
}
