package wtapp

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mukeshmk/devex-manager/internal/shared/config"
	"github.com/mukeshmk/devex-manager/internal/shared/gitx"
	"github.com/mukeshmk/devex-manager/internal/shared/testutil"
)

// addApp is an app on a fixture whose origin has branch r1, fetched; conf, if set, is the .devex.conf.
func addApp(t *testing.T, conf string) (*testutil.Fixture, tea.Model) {
	t.Helper()
	f := testutil.NewRepo(t)
	gitx.Run(filepath.Join(f.Tmp, "seed"), "push", "-q", f.Origin, "main:refs/heads/r1", "main:refs/heads/feature/a-very-long-remote-branch-name-that-cannot-fit-in-sixty-columns")
	if _, err := gitx.Run(f.Root, "fetch", "-q", "origin"); err != nil {
		t.Fatal(err)
	}
	if conf != "" {
		write(t, filepath.Join(f.Root, ".devex.conf"), conf)
	}
	repo, err := gitx.Open(f.Root)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadRepo(repo.Root)
	if err != nil {
		t.Fatal(err)
	}
	mm := New(repo, cfg, filepath.Join(f.Root, "main")).(model)
	var m tea.Model = mm
	m, _ = m.Update(mm.load()())
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return f, m
}

// submit presses enter and follows the commands a local popup's pick returns, up to starting the flow.
func submit(m tea.Model) tea.Model {
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	for cmd != nil {
		msg := cmd()
		if msg == nil {
			break
		}
		m, cmd = m.Update(msg)
	}
	return m
}

// finish drains the flow and applies the reload that follows it.
func finish(t *testing.T, m tea.Model) tea.Model {
	t.Helper()
	m = drain(t, m, 200)
	m, _ = m.Update(m.(model).load()())
	return m
}

func TestAppAddRemoteBranch(t *testing.T) {
	f, m := addApp(t, "")
	m = press(m, "n")
	p := m.(model).popup
	if p == nil || !p.filterable || p.pinned != 1 || p.options[0] != "+ new branch…" || !slices.Contains(p.options, "r1") {
		t.Fatalf("want filterable branch popup, got %+v", p)
	}
	checkWidth(t, resize(press(m, "a"), 60, 12).View(), 60, 12)
	m = finish(t, submit(press(m, "r1")))
	mm := m.(model)
	if !exists(filepath.Join(f.Root, "r1")) || !strings.Contains(mm.statusMsg, "add: done") {
		t.Fatalf("folder exists=%v status %q", exists(filepath.Join(f.Root, "r1")), mm.statusMsg)
	}
	if r, ok := mm.current(); !ok || r.WT.Branch != "r1" {
		t.Errorf("selected %+v", r.WT)
	}
}

func TestAppAddNewBranch(t *testing.T) {
	f, m := addApp(t, "")
	f.AddWorktree(t, "zzz", "zzz")
	m = press(m, "r", "down") // select zzz, so landing on brand-new is not the kept selection
	if r, _ := m.(model).current(); r.WT.Branch != "zzz" {
		t.Fatalf("setup selected %+v", r.WT)
	}
	m = submit(press(m, "n"))
	if p := m.(model).popup; p == nil || p.kind != popupInput || p.title != "New branch (from main)" {
		t.Fatalf("want new branch input, got %+v", p)
	}
	m = submit(m) // empty name does nothing
	if mm := m.(model); mm.busy != "" || mm.popup != nil {
		t.Fatalf("empty name: busy %q popup %+v", mm.busy, mm.popup)
	}
	m = finish(t, submit(press(submit(press(m, "n")), "brand-new")))
	base, _ := gitx.Run(f.Root, "rev-parse", "main")
	if tip, err := gitx.Run(f.Root, "rev-parse", "refs/heads/brand-new"); err != nil || tip != base {
		t.Errorf("brand-new %q %v, want %q", tip, err, base)
	}
	if !exists(filepath.Join(f.Root, "brand-new")) {
		t.Errorf("no folder; status %q", m.(model).statusMsg)
	}
	if r, _ := m.(model).current(); r.WT.Path != filepath.Join(f.Root, "brand-new") {
		t.Errorf("selected %+v", r.WT)
	}
}

func TestAppAddCustomFolder(t *testing.T) {
	f, m := addApp(t, "[worktree]\nnaming_strategy = custom\n")
	m = submit(press(m, "n", "r1"))
	if p := m.(model).popup; p == nil || p.kind != popupInput || p.title != "Folder name for r1" {
		t.Fatalf("want folder input, got %+v", p)
	}
	m = submit(m) // empty folder does nothing
	if mm := m.(model); mm.busy != "" || mm.popup != nil {
		t.Fatalf("empty folder: busy %q popup %+v", mm.busy, mm.popup)
	}
	m = finish(t, submit(press(submit(press(m, "n", "r1")), "my-r1")))
	if !exists(filepath.Join(f.Root, "my-r1")) {
		t.Errorf("no my-r1; status %q", m.(model).statusMsg)
	}
}

func TestAppFetch(t *testing.T) {
	f, m := addApp(t, "")
	gitx.Run(filepath.Join(f.Tmp, "seed"), "push", "-q", f.Origin, "main:refs/heads/late")
	m = press(m, "f")
	if m.(model).busy != "fetch" {
		t.Fatalf("busy %q", m.(model).busy)
	}
	m = finish(t, m)
	if s := m.(model).statusMsg; !strings.Contains(s, "fetch: done") {
		t.Fatalf("status %q", s)
	}
	if p := press(m, "n").(model).popup; p == nil || !slices.Contains(p.options, "late") {
		t.Errorf("late not listed: %+v", p)
	}
}

func TestAppAddIgnoredWhileBusy(t *testing.T) {
	_, m := addApp(t, "")
	mm := m.(model)
	mm.busy = "sync"
	m = press(mm, "n", "f")
	if mm := m.(model); mm.busy != "sync" || mm.popup != nil {
		t.Errorf("busy keys acted: busy %q popup %+v", mm.busy, mm.popup)
	}
	// A branch list that lands after a flow started opens nothing.
	m, _ = m.Update(branchesMsg{branches: []string{"r1"}})
	if p := m.(model).popup; p != nil {
		t.Errorf("stale branch list opened %+v", p)
	}
}

// An add pick whose message lands after queued keys started a flow must not start a second flow.
func TestAppAddStaleIgnored(t *testing.T) {
	f, m := addApp(t, "")
	mm := m.(model)
	mm.busy = "sync"
	m, cmd := mm.Update(addMsg{branch: "r1"})
	if mm := m.(model); mm.busy != "sync" || cmd != nil || mm.selectBranch != "" {
		t.Fatalf("stale add acted: busy %q cmd %v", mm.busy, cmd != nil)
	}
	mm = m.(model)
	mm.busy = ""
	mm.popup = &popup{kind: popupHelp}
	if m, _ = mm.Update(addMsg{}); m.(model).popup.kind != popupHelp {
		t.Errorf("stale add replaced the open popup: %+v", m.(model).popup)
	}
	if exists(filepath.Join(f.Root, "r1")) {
		t.Error("r1 added")
	}
}

// q quit and ? keys always stay on the hint line.
func TestAppFooterKeepsQuit(t *testing.T) {
	_, m := addApp(t, "")
	for _, w := range []int{60, 80, 100, 200} {
		for _, k := range []string{"1", "2", "3", "4", "5"} {
			v := resize(press(m, k), w, 24).View()
			l := strings.Split(v, "\n")
			foot := l[len(l)-1]
			if !strings.Contains(foot, "q quit") || !strings.Contains(foot, "? keys") {
				t.Errorf("%d cols, panel %s: %q", w, k, foot)
			}
			checkWidth(t, v, w, 24)
		}
	}
}

func TestAppAddBranchListError(t *testing.T) {
	_, m := addApp(t, "")
	m, _ = m.Update(branchesMsg{err: errors.New("boom")})
	if mm := m.(model); mm.popup != nil || !mm.statusErr || mm.statusMsg != "boom" {
		t.Errorf("popup %+v status %q err %v", mm.popup, mm.statusMsg, mm.statusErr)
	}
}
