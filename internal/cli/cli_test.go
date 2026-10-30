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
