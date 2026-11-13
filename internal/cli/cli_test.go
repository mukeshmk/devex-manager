package cli

import (
	"bytes"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestPassthrough(t *testing.T) {
	cases := []struct {
		in   []string
		want []string
		ok   bool
	}{
		{[]string{"lock", "x"}, []string{"worktree", "lock", "x"}, true},
		{[]string{"list"}, nil, false},
		{[]string{"list", "--porcelain"}, []string{"worktree", "list", "--porcelain"}, true},
		{[]string{"add", "foo"}, nil, false},
	}
	for _, c := range cases {
		got, ok := Passthrough(c.in)
		if ok != c.ok || !reflect.DeepEqual(got, c.want) {
			t.Errorf("Passthrough(%v) = %v,%v want %v,%v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestMainUnknownPrintsHelp(t *testing.T) {
	in, _ := os.CreateTemp(t.TempDir(), "in")
	defer in.Close()
	var out, errw bytes.Buffer
	if rc := Run("git-wt", []string{"bogus"}, in, &out, &errw); rc != 1 {
		t.Fatalf("git-wt rc=%d", rc)
	}
	if !strings.Contains(out.String()+errw.String(), "Usage: git wt <command> [arguments]") {
		t.Errorf("git-wt help missing: %q", out.String()+errw.String())
	}
	out.Reset()
	errw.Reset()
	if rc := Run("dx", []string{"bogus"}, in, &out, &errw); rc != 1 {
		t.Fatalf("dx rc=%d", rc)
	}
	all := out.String() + errw.String()
	for _, w := range []string{"skills", "nb", "ctx"} {
		if !strings.Contains(all, w) {
			t.Errorf("dx help missing %q: %q", w, all)
		}
	}
}

// Bare `git wt` opens the app only on a terminal; non-TTY keeps help and exit 1.
func TestBareGitWtNonTTYPrintsHelp(t *testing.T) {
	in, _ := os.CreateTemp(t.TempDir(), "in")
	defer in.Close()
	var out, errw bytes.Buffer
	if rc := Run("git-wt", nil, in, &out, &errw); rc != 1 {
		t.Fatalf("rc=%d", rc)
	}
	if !strings.Contains(out.String(), "Usage: git wt <command> [arguments]") || strings.Contains(out.String(), "\x1b[?1049h") {
		t.Errorf("out %q", out.String())
	}
}

func TestVersionBothRoots(t *testing.T) {
	in, _ := os.CreateTemp(t.TempDir(), "in")
	defer in.Close()
	for _, argv0 := range []string{"dx", "git-wt"} {
		var out, errw bytes.Buffer
		if rc := Run(argv0, []string{"--version"}, in, &out, &errw); rc != 0 || !strings.Contains(out.String(), Version) {
			t.Errorf("%s --version: rc=%d out=%q err=%q", argv0, rc, out.String(), errw.String())
		}
	}
}

func TestGitWtHelpListsFlags(t *testing.T) {
	in, _ := os.CreateTemp(t.TempDir(), "in")
	defer in.Close()
	var out bytes.Buffer
	Run("git-wt", []string{"--help"}, in, &out, &out)
	for _, w := range []string{"--check", "--dry-run", "--yes", "--force"} {
		if !strings.Contains(out.String(), w) {
			t.Errorf("help missing %q:\n%s", w, out.String())
		}
	}
}

func TestCloneRejectsExtraArgs(t *testing.T) {
	t.Chdir(t.TempDir())
	in, _ := os.CreateTemp(t.TempDir(), "in")
	defer in.Close()
	var out bytes.Buffer
	if rc := Run("git-wt", []string{"clone", "a", "b"}, in, &out, &out); rc != 1 || !strings.Contains(out.String(), "Too many arguments") {
		t.Errorf("rc=%d out=%q", rc, out.String())
	}
}
