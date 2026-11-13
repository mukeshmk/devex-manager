package wtsync

import (
	"reflect"
	"testing"
)

const (
	M  = "/r/main"
	F  = "/r/feat"
	F2 = "/r/feat2"
	F3 = "/r/feat3"
)

var others = []string{F, F2}

const (
	settings = ".vscode/settings.json"
	launch   = ".vscode/launch.json"
)

// world is an in-memory filesystem: abs path -> content.
type world struct {
	t     *testing.T
	files map[string]string
}

func newWorld(t *testing.T, files map[string]string) *world { return &world{t, files} }

func (w *world) same(a, b string) bool {
	x, ok1 := w.files[a]
	y, ok2 := w.files[b]
	return ok1 && ok2 && x == y
}

func (w *world) apply(ops []Op) {
	for _, op := range ops {
		switch op.Kind {
		case Copy:
			c, ok := w.files[op.Src.Path()]
			if !ok {
				w.t.Fatalf("copy from missing %s", op.Src.Path())
			}
			w.files[op.Dst.Path()] = c
		case Delete:
			delete(w.files, op.Dst.Path())
		}
	}
}

func (w *world) want(path, content string) {
	w.t.Helper()
	if got, ok := w.files[path]; !ok || got != content {
		w.t.Fatalf("%s = %q (exists %v), want %q", path, got, ok, content)
	}
}

func (w *world) gone(paths ...string) {
	w.t.Helper()
	for _, p := range paths {
		if _, ok := w.files[p]; ok {
			w.t.Fatalf("%s should not exist", p)
		}
	}
}

func s(mtime, size int64) Stat { return Stat{Exists: true, Mtime: mtime, Size: size} }

var none = Stat{}

func fstate(rel string, rec, main Stat, o map[string]Stat) FileState {
	return FileState{Rel: rel, Recorded: rec, Main: main, Others: o}
}

// picks answers choices in order and fails if asked more often than expected.
func picks(t *testing.T, answers ...int) func(Step) int {
	return func(st Step) int {
		t.Helper()
		if len(answers) == 0 {
			t.Fatalf("unexpected question %q", st.Choice.Question)
		}
		a := answers[0]
		answers = answers[1:]
		return a
	}
}

func run(w *world, files []FileState, pick func(Step) int, o ...string) Plan {
	if len(o) == 0 {
		o = others
	}
	p := BuildPlan(M, o, files, w.same)
	w.apply(Walk(p.Steps, pick))
	return p
}

func TestPlanAdditions(t *testing.T) {
	// TEST 1: settings.json added in main, launch.json added in feat, no manifest yet.
	w := newWorld(t, map[string]string{M + "/" + settings: "main-settings", F + "/" + launch: "feat-launch"})
	p := run(w, []FileState{
		fstate(launch, none, none, map[string]Stat{F: s(100, 12), F2: none}),
		fstate(settings, none, s(100, 14), map[string]Stat{F: none, F2: none}),
	}, Defaults)
	w.want(F+"/"+settings, "main-settings")
	w.want(F2+"/"+settings, "main-settings")
	w.want(M+"/"+launch, "feat-launch")
	w.want(F2+"/"+launch, "feat-launch")
	for _, st := range p.Steps {
		if st.Choice != nil {
			t.Fatalf("additions must not ask: %+v", st)
		}
	}
	if p.Steps[1].Summary != "update .vscode/settings.json in feat, feat2" {
		t.Fatalf("summary %q", p.Steps[1].Summary)
	}
}

func TestPlanModifications(t *testing.T) {
	// TEST 2a: main modified, worktrees at the recorded state.
	w := newWorld(t, map[string]string{M + "/" + settings: "main-settings-v2", F + "/" + settings: "main-settings", F2 + "/" + settings: "main-settings"})
	run(w, []FileState{fstate(settings, s(100, 14), s(101, 17), map[string]Stat{F: s(100, 14), F2: s(100, 14)})}, picks(t))
	w.want(F+"/"+settings, "main-settings-v2")
	w.want(F2+"/"+settings, "main-settings-v2")

	// TEST 2b: feat modified, main untouched.
	w = newWorld(t, map[string]string{M + "/" + launch: "feat-launch", F + "/" + launch: "feat-launch-v2", F2 + "/" + launch: "feat-launch"})
	run(w, []FileState{fstate(launch, s(100, 12), s(100, 12), map[string]Stat{F: s(102, 15), F2: s(100, 12)})}, picks(t))
	w.want(M+"/"+launch, "feat-launch-v2")
	w.want(F2+"/"+launch, "feat-launch-v2")
}

func TestPlanUntouchedButDifferentIsModifying(t *testing.T) {
	// Same mtime and size as recorded, but content differs from an unmodified main (git-wt-sync:330-334).
	w := newWorld(t, map[string]string{M + "/" + launch: "aaaa", F + "/" + launch: "bbbb", F2 + "/" + launch: "aaaa"})
	run(w, []FileState{fstate(launch, s(100, 4), s(100, 4), map[string]Stat{F: s(100, 4), F2: s(100, 4)})}, picks(t))
	w.want(M+"/"+launch, "bbbb")
	w.want(F2+"/"+launch, "bbbb")
}

func launchRemovedInFeat(t *testing.T) (*world, []FileState) {
	w := newWorld(t, map[string]string{M + "/" + launch: "L", F2 + "/" + launch: "L"})
	return w, []FileState{fstate(launch, s(100, 1), s(100, 1), map[string]Stat{F: none, F2: s(100, 1)})}
}

func TestPlanRemovedInWT(t *testing.T) {
	// TEST 3 A: answer n (keep in main), then y (restore in feat).
	w, files := launchRemovedInFeat(t)
	p := run(w, files, picks(t, 1, 0))
	st := p.Steps[0]
	c := st.Choice
	if c == nil || !c.YesNo || c.Default != 1 || !c.BreakOnYes ||
		c.Question != "File .vscode/launch.json was removed in feat. Remove it from main as well?" ||
		!reflect.DeepEqual(c.Options, []string{"yes", "no"}) {
		t.Fatalf("remove step %+v", c)
	}
	if len(st.Then) != 1 || st.Then[0].Choice == nil || !st.Then[0].Choice.YesNo || st.Then[0].Choice.Default != 0 ||
		st.Then[0].Choice.Question != "Restore File .vscode/launch.json in feat?" {
		t.Fatalf("restore step %+v", st.Then)
	}
	w.want(F+"/"+launch, "L")
	w.want(M+"/"+launch, "L")

	// Non-interactive defaults: No, then restore Yes.
	w, files = launchRemovedInFeat(t)
	run(w, files, Defaults)
	w.want(F+"/"+launch, "L")

	// TEST 3 B: answer y, deleted everywhere, no restore question.
	w, files = launchRemovedInFeat(t)
	run(w, files, picks(t, 0))
	w.gone(M+"/"+launch, F+"/"+launch, F2+"/"+launch)
}

func TestPlanRemovedInSeveralWTsBreaksOnYes(t *testing.T) {
	w := newWorld(t, map[string]string{M + "/" + launch: "L"})
	files := []FileState{fstate(launch, s(100, 1), s(100, 1), map[string]Stat{F: none, F2: none})}
	p := BuildPlan(M, others, files, w.same)
	if len(p.Steps) != 2 {
		t.Fatalf("want one step per deleting worktree, got %+v", p.Steps)
	}
	// yes on feat: feat2's question is never asked (picks fails on a third call).
	w.apply(Walk(p.Steps, picks(t, 0)))
	w.gone(M + "/" + launch)
	// no+no on feat, then yes on feat2.
	w = newWorld(t, map[string]string{M + "/" + launch: "L"})
	w.apply(Walk(p.Steps, picks(t, 1, 1, 0)))
	w.gone(M+"/"+launch, F+"/"+launch)
}

func TestPlanRemovedInWTModifiedInMain(t *testing.T) {
	mk := func() (*world, []FileState) {
		w := newWorld(t, map[string]string{M + "/" + launch: "new", F2 + "/" + launch: "old"})
		return w, []FileState{fstate(launch, s(100, 3), s(105, 3), map[string]Stat{F: none, F2: s(100, 3)})}
	}
	w, files := mk()
	p := run(w, files, Defaults)
	if c := p.Steps[0].Choice; c.Default != 2 || c.Question != "Conflict in .vscode/launch.json. Which version should be kept?" ||
		!reflect.DeepEqual(c.Options, []string{"Keep Main version", "Keep feat version", "Resolve Later"}) {
		t.Fatalf("choice %+v", c)
	}
	w.want(M+"/"+launch+".devex-conflict-feat-main", "new")
	w, files = mk()
	run(w, files, picks(t, 0))
	w.want(F+"/"+launch, "new")
	w, files = mk()
	run(w, files, picks(t, 1))
	w.gone(M+"/"+launch, F2+"/"+launch)
}

func TestPlanRemovedInMain(t *testing.T) {
	// TEST 4: deleted in main, untouched elsewhere: silently removed.
	w := newWorld(t, map[string]string{F + "/" + settings: "x", F2 + "/" + settings: "x"})
	p := run(w, []FileState{fstate(settings, s(100, 1), none, map[string]Stat{F: s(100, 1), F2: s(100, 1)})}, picks(t))
	w.gone(F+"/"+settings, F2+"/"+settings)
	if p.Steps[0].Choice != nil {
		t.Fatal("should not ask")
	}
}

func TestPlanRemovedInMainModifiedInWT(t *testing.T) {
	mk := func() (*world, []FileState) {
		w := newWorld(t, map[string]string{F + "/" + settings: "feat-v", F2 + "/" + settings: "x"})
		return w, []FileState{fstate(settings, s(100, 1), none, map[string]Stat{F: s(103, 6), F2: s(100, 1)})}
	}
	w, files := mk()
	run(w, files, Defaults)
	w.want(M+"/"+settings+".devex-conflict-feat", "feat-v")
	w.gone(M + "/" + settings)
	w, files = mk()
	run(w, files, picks(t, 0))
	w.gone(F + "/" + settings)
	w.want(F2+"/"+settings, "x") // Bash leaves untouched worktrees alone here
	w, files = mk()
	run(w, files, picks(t, 1))
	w.want(M+"/"+settings, "feat-v")
	w.want(F2+"/"+settings, "feat-v")
}

func conflictMainAndFeat(t *testing.T) (*world, []FileState) {
	w := newWorld(t, map[string]string{M + "/" + settings: "main-conflict", F + "/" + settings: "feat-conflict", F2 + "/" + settings: "base-settings"})
	return w, []FileState{fstate(settings, s(100, 14), s(102, 14), map[string]Stat{F: s(102, 14), F2: s(100, 14)})}
}

func TestPlanConflictMainAndWT(t *testing.T) {
	// TEST 5: Resolve Later (default) writes the conflict file, main untouched.
	w, files := conflictMainAndFeat(t)
	p := run(w, files, Defaults)
	c := p.Steps[0].Choice
	if c == nil || c.Default != 2 || c.YesNo || c.Question != "Conflict in .vscode/settings.json. Which version should be kept?" ||
		!reflect.DeepEqual(c.Options, []string{"Keep Main version", "Keep feat version", "Resolve Later"}) {
		t.Fatalf("choice %+v", c)
	}
	if p.Steps[0].Summary != "conflict: .vscode/settings.json modified in main and feat" {
		t.Fatalf("summary %q", p.Steps[0].Summary)
	}
	w.want(M+"/"+settings, "main-conflict")
	w.want(M+"/"+settings+".devex-conflict-feat", "feat-conflict")
	w.want(F2+"/"+settings, "main-conflict") // M1: untouched feat2 is not left stale

	// Keep feat version: main and feat2 get it.
	w, files = conflictMainAndFeat(t)
	run(w, files, picks(t, 1))
	w.want(M+"/"+settings, "feat-conflict")
	w.want(F2+"/"+settings, "feat-conflict")

	// Keep main version.
	w, files = conflictMainAndFeat(t)
	run(w, files, picks(t, 0))
	w.want(F+"/"+settings, "main-conflict")
	w.want(F2+"/"+settings, "main-conflict")
}

func TestPlanBothModifiedIdenticallyAligns(t *testing.T) {
	w := newWorld(t, map[string]string{M + "/" + settings: "same", F + "/" + settings: "same", F2 + "/" + settings: "old"})
	p := run(w, []FileState{fstate(settings, s(100, 3), s(102, 4), map[string]Stat{F: s(103, 4), F2: s(100, 3)})}, picks(t))
	want := []Op{{Kind: Copy, Src: Loc{M, settings}, Dst: Loc{F, settings}}}
	if len(p.Steps) != 2 || p.Steps[0].Choice != nil || !reflect.DeepEqual(p.Steps[0].Ops, want) {
		t.Fatalf("steps %+v", p.Steps)
	}
	w.want(F2+"/"+settings, "same") // M1: untouched feat2 gets main's change too
}

func TestPlanConflictMultiWT(t *testing.T) {
	// TEST 6: feat and feat2 modified differently, main untouched.
	mk := func() (*world, []FileState) {
		w := newWorld(t, map[string]string{M + "/" + settings: "base-settings-v3", F + "/" + settings: "feat-val", F2 + "/" + settings: "feat2-val"})
		return w, []FileState{fstate(settings, s(100, 17), s(100, 17), map[string]Stat{F: s(102, 9), F2: s(102, 10)})}
	}
	w, files := mk()
	p := run(w, files, Defaults)
	c := p.Steps[0].Choice
	if c == nil || c.Default != 2 || c.Question != "Conflict in .vscode/settings.json. Modified in feat and feat2. Which version to keep?" ||
		!reflect.DeepEqual(c.Options, []string{"Keep feat", "Keep feat2", "Resolve Later"}) {
		t.Fatalf("choice %+v", c)
	}
	w.want(M+"/"+settings+".devex-conflict-feat", "feat-val")
	w.want(M+"/"+settings+".devex-conflict-feat2", "feat2-val")
	w.want(M+"/"+settings, "base-settings-v3")

	w, files = mk()
	run(w, files, picks(t, 1))
	w.want(M+"/"+settings, "feat2-val")
	w.want(F+"/"+settings, "feat2-val")
}

func TestPlanIdenticalAdditions(t *testing.T) {
	w := newWorld(t, map[string]string{F + "/.env": "A=1", F2 + "/.env": "A=1"})
	p := run(w, []FileState{fstate(".env", none, none, map[string]Stat{F: s(100, 3), F2: s(101, 3)})}, picks(t))
	w.want(M+"/.env", "A=1")
	if p.Steps[0].Choice != nil {
		t.Fatal("identical additions must not ask")
	}
	// Different additions ask, keeping the first by option 1.
	w = newWorld(t, map[string]string{F + "/.env": "A=1", F2 + "/.env": "A=2"})
	run(w, []FileState{fstate(".env", none, none, map[string]Stat{F: s(100, 3), F2: s(101, 3)})}, picks(t, 0))
	w.want(M+"/.env", "A=1")
	w.want(F2+"/.env", "A=1")
}

func TestPlanMissingInNewWorktree(t *testing.T) {
	// A recorded file absent from a new worktree counts as deleting there (git-wt-sync:344-345),
	// so it goes through the R2 questions; the defaults (No, then restore Yes) populate it.
	w := newWorld(t, map[string]string{M + "/.env": "A=1", F + "/.env": "A=1", F2 + "/.env": "A=1"})
	p := run(w, []FileState{fstate(".env", s(100, 3), s(100, 3), map[string]Stat{F: s(100, 3), F2: s(100, 3), F3: none})}, Defaults, F, F2, F3)
	w.want(F3+"/.env", "A=1")
	if len(p.Steps) != 1 || p.Steps[0].Choice == nil || !p.Steps[0].Choice.YesNo {
		t.Fatalf("steps %+v", p.Steps)
	}
	// Not yet in the manifest: main counts as modified and fans out to the missing worktree.
	w = newWorld(t, map[string]string{M + "/.env": "A=1"})
	run(w, []FileState{fstate(".env", none, s(100, 3), map[string]Stat{F: none, F2: none, F3: none})}, picks(t), F, F2, F3)
	w.want(F3+"/.env", "A=1")
}

func TestPlanInSyncHasNoSteps(t *testing.T) {
	w := newWorld(t, map[string]string{M + "/.env": "A", F + "/.env": "A", F2 + "/.env": "A"})
	files := []FileState{fstate(".env", s(100, 1), s(100, 1), map[string]Stat{F: s(100, 1), F2: s(100, 1)})}
	if p := BuildPlan(M, others, files, w.same); len(p.Steps) != 0 || p.Main != M {
		t.Fatalf("plan %+v", p)
	}
	if lines := CheckOutOfSync(M, others, files, w.same); len(lines) != 0 {
		t.Fatalf("lines %q", lines)
	}
}

func TestCheckOutOfSyncWording(t *testing.T) {
	w := newWorld(t, map[string]string{
		M + "/" + launch: "L", F + "/" + launch: "L2", F2 + "/" + launch: "L",
		F + "/" + settings: "S", M + "/.env": "E",
	})
	files := []FileState{
		fstate(".env", s(100, 1), s(100, 1), map[string]Stat{F: none, F2: none}),
		fstate(launch, s(100, 1), s(100, 1), map[string]Stat{F: s(102, 2), F2: s(100, 1)}),
		fstate(settings, s(100, 1), none, map[string]Stat{F: s(100, 1), F2: none}),
	}
	got := CheckOutOfSync(M, others, files, w.same)
	want := []string{
		"Out of sync: .env",
		"    - Deleted in: feat feat2",
		"Out of sync: .vscode/launch.json",
		"    - Modified in: feat",
		"Out of sync: .vscode/settings.json",
		"    - Deleted in main worktree",
		"    - Deleted in: feat2",
		"    - New file in: ",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got\n%q\nwant\n%q", got, want)
	}
	files = []FileState{fstate(".env", none, s(100, 1), map[string]Stat{F: s(100, 1), F2: none})}
	got = CheckOutOfSync(M, others, files, w.same)
	want = []string{"Out of sync: .env", "    - Modified in main worktree", "    - Modified in: feat", "    - Missing in: feat2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q want %q", got, want)
	}
}
