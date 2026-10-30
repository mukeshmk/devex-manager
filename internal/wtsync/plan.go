package wtsync

import (
	"path/filepath"
	"slices"
	"strings"
)

type OpKind int

const (
	Copy   OpKind = iota // Src -> Dst, creating parent dirs and preserving mode and mtime (`cp -p`)
	Delete               // remove Dst if present (`rm -f`); Src unused
)

// Loc is a file Rel (forward slashes) inside worktree WT.
type Loc struct{ WT, Rel string }

func (l Loc) Path() string { return filepath.Join(l.WT, l.Rel) }

type Op struct {
	Kind     OpKind
	Src, Dst Loc
}

// Choice is a question; Ops[i] is the op list for Options[i] and Default indexes Options.
// YesNo choices have Options ["yes","no"]. BreakOnYes means taking "yes" skips the remaining
// Steps for the same Rel (the Bash `break` in the removed-in-worktree loop).
type Choice struct {
	Question   string
	Options    []string
	Default    int
	Ops        [][]Op
	YesNo      bool
	BreakOnYes bool
	// RecheckSame (R4 conflicts) names main and the worktree file: if earlier answers already made
	// them identical, Resolve takes Ops[0] (align main->wt) without asking, like the Bash runtime diff.
	RecheckSame *[2]Loc
}

// Step is the work for one file: Ops run unconditionally, then Choice (if any) is asked.
// Then holds follow-up steps walked only when "no" (index 1) of a YesNo Choice is taken.
type Step struct {
	Rel     string
	Summary string
	Ops     []Op
	Choice  *Choice
	Then    []Step
}

type Plan struct {
	Main  string
	Steps []Step
}

// Defaults picks each Choice's default, which is the non-interactive behaviour of git-wt-sync.
func Defaults(s Step) int { return s.Choice.Default }

// Walk resolves steps in order with pick and returns the ops to apply, in order.
func Walk(steps []Step, pick func(Step) int) []Op {
	return walk(steps, func(s Step, _ []Op) int { return pick(s) }, nil)
}

// walk appends to ops and hands pick the ops resolved so far.
func walk(steps []Step, pick func(Step, []Op) int, ops []Op) []Op {
	broken := "" // steps for one file are contiguous, so the last broken Rel is enough
	for _, s := range steps {
		if broken != "" && s.Rel == broken {
			continue
		}
		ops = append(ops, s.Ops...)
		if s.Choice == nil {
			continue
		}
		i := pick(s, ops)
		ops = append(ops, s.Choice.Ops[i]...)
		if s.Choice.YesNo && i == 1 {
			ops = walk(s.Then, pick, ops)
		}
		if s.Choice.BreakOnYes && i == 0 {
			broken = s.Rel
		}
	}
	return ops
}

// class is the per-file classification of git-wt-sync:280-350.
type class struct {
	existsMain, mainModified, mainDeleted   bool
	modifying, deleting, untouched, missing []string
}

func classify(main string, others []string, f FileState, same func(a, b string) bool) class {
	recOK := f.Recorded.Mtime != 0
	c := class{existsMain: f.Main.Exists}
	if f.Main.Exists {
		c.mainModified = !recOK || f.Main.Mtime != f.Recorded.Mtime || f.Main.Size != f.Recorded.Size
	} else {
		c.mainDeleted = recOK
	}
	for _, wt := range others {
		st := f.Others[wt]
		switch {
		case st.Exists:
			mod := !recOK
			if recOK && st.Mtime >= f.Recorded.Mtime {
				mod = st.Mtime > f.Recorded.Mtime || st.Size != f.Recorded.Size
				if !mod && c.existsMain && !c.mainModified {
					mod = !same(filepath.Join(wt, f.Rel), filepath.Join(main, f.Rel))
				}
			}
			if mod {
				c.modifying = append(c.modifying, wt)
			} else {
				c.untouched = append(c.untouched, wt)
			}
		case recOK:
			c.deleting = append(c.deleting, wt)
		default:
			c.missing = append(c.missing, wt)
		}
	}
	return c
}

// fanOut copies src to Rel in every worktree of all except the listed ones.
func fanOut(src Loc, all []string, except ...string) []Op {
	var ops []Op
	for _, w := range all {
		if !slices.Contains(except, w) {
			ops = append(ops, Op{Kind: Copy, Src: src, Dst: Loc{w, src.Rel}})
		}
	}
	return ops
}

func deleteIn(rel string, wts ...string) []Op {
	var ops []Op
	for _, w := range wts {
		ops = append(ops, Op{Kind: Delete, Dst: Loc{w, rel}})
	}
	return ops
}

func cp(from, to, rel string) Op { return Op{Kind: Copy, Src: Loc{from, rel}, Dst: Loc{to, rel}} }

func names(wts []string, sep string) string {
	var n []string
	for _, w := range wts {
		n = append(n, filepath.Base(w))
	}
	return strings.Join(n, sep)
}

func dsts(ops []Op) []string {
	var w []string
	for _, op := range ops {
		if !slices.Contains(w, op.Dst.WT) {
			w = append(w, op.Dst.WT)
		}
	}
	return w
}

func autoStep(rel string, ops []Op) Step {
	verb := "update"
	if ops[0].Kind == Delete {
		verb = "delete"
	}
	return Step{Rel: rel, Summary: verb + " " + rel + " in " + names(dsts(ops), ", "), Ops: ops}
}

func conflictCopy(from, main, rel, suffix string) Op {
	return Op{Kind: Copy, Src: Loc{from, rel}, Dst: Loc{main, rel + conflictTag + suffix}}
}

// mainVsWT is prompt_conflict: Keep Main / Keep <wt> / Resolve Later.
func mainVsWT(rel, summary, wt string, keepMain, keepWT, later []Op) Step {
	return Step{Rel: rel, Summary: summary, Choice: &Choice{
		Question: "Conflict in " + rel + ". Which version should be kept?",
		Options:  []string{"Keep Main version", "Keep " + filepath.Base(wt) + " version", "Resolve Later"},
		Default:  2,
		Ops:      [][]Op{keepMain, keepWT, later},
	}}
}

func yesNo(question string, def int, yes []Op) *Choice {
	return &Choice{Question: question, Options: []string{"yes", "no"}, Default: def, Ops: [][]Op{yes, nil}, YesNo: true}
}

// fromModifying covers "one or more worktrees changed it, main didn't" (R3 and R5).
// keepFirstToo: R3's identical-additions fan-out includes the first worktree (git-wt-sync:483-485), R5's does not (576-578).
func fromModifying(main string, all []string, rel string, mod []string, same func(a, b string) bool, keepFirstToo bool) []Step {
	m := Loc{main, rel}
	switch {
	case len(mod) == 1:
		return []Step{autoStep(rel, append([]Op{cp(mod[0], main, rel)}, fanOut(m, all, mod[0], main)...))}
	case len(mod) > 1:
		first := filepath.Join(mod[0], rel)
		identical := true
		for _, w := range mod[1:] {
			if !same(first, filepath.Join(w, rel)) {
				identical = false
				break
			}
		}
		if identical {
			except := []string{mod[0], main}
			if keepFirstToo {
				except = except[1:]
			}
			return []Step{autoStep(rel, append([]Op{cp(mod[0], main, rel)}, fanOut(m, all, except...)...))}
		}
		a, b := mod[0], mod[1]
		var later []Op
		for _, w := range mod {
			later = append(later, conflictCopy(w, main, rel, filepath.Base(w)))
		}
		return []Step{{Rel: rel, Summary: "conflict: " + rel + " modified in " + filepath.Base(a) + " and " + filepath.Base(b), Choice: &Choice{
			Question: "Conflict in " + rel + ". Modified in " + filepath.Base(a) + " and " + filepath.Base(b) + ". Which version to keep?",
			Options:  []string{"Keep " + filepath.Base(a), "Keep " + filepath.Base(b), "Resolve Later"},
			Default:  2,
			Ops: [][]Op{
				append([]Op{cp(a, main, rel)}, fanOut(m, all, a, main)...),
				append([]Op{cp(b, main, rel)}, fanOut(m, all, b, main)...),
				later,
			},
		}}}
	}
	return nil
}

func planFile(main string, others []string, f FileState, same func(a, b string) bool) []Step {
	all := append([]string{main}, others...)
	rel, m := f.Rel, Loc{main, f.Rel}
	c := classify(main, others, f, same)
	var steps []Step
	switch {
	case c.mainDeleted: // R1, git-wt-sync:382-410
		if len(c.modifying) == 0 {
			if len(c.untouched) > 0 {
				steps = append(steps, autoStep(rel, deleteIn(rel, c.untouched...)))
			}
			break
		}
		for _, wt := range c.modifying {
			steps = append(steps, mainVsWT(rel, "conflict: "+rel+" deleted in main and modified in "+filepath.Base(wt), wt,
				deleteIn(rel, wt),
				append([]Op{cp(wt, main, rel)}, fanOut(m, all, wt, main)...),
				[]Op{conflictCopy(wt, main, rel, filepath.Base(wt))}))
		}
	case len(c.deleting) > 0: // R2, git-wt-sync:412-455
		if c.mainModified {
			for _, wt := range c.deleting {
				var rest []string
				for _, w := range all {
					if w != wt && w != main {
						rest = append(rest, w)
					}
				}
				steps = append(steps, mainVsWT(rel, "conflict: "+rel+" modified in main and deleted in "+filepath.Base(wt), wt,
					[]Op{cp(main, wt, rel)},
					deleteIn(rel, append([]string{main}, rest...)...),
					[]Op{conflictCopy(main, main, rel, filepath.Base(wt)+"-main")}))
			}
			break
		}
		for _, wt := range c.deleting {
			n := filepath.Base(wt)
			ask := yesNo("File "+rel+" was removed in "+n+". Remove it from main as well?", 1, deleteIn(rel, all...))
			ask.BreakOnYes = true
			restore := Step{Rel: rel, Summary: "restore " + rel + " in " + n,
				Choice: yesNo("Restore File "+rel+" in "+n+"?", 0, []Op{cp(main, wt, rel)})}
			steps = append(steps, Step{Rel: rel, Summary: "removed: " + rel + " in " + n, Choice: ask, Then: []Step{restore}})
		}
	case !c.existsMain: // R3, git-wt-sync:457-512
		steps = fromModifying(main, all, rel, c.modifying, same, true)
	case c.mainModified: // R4, git-wt-sync:516-551
		if len(c.modifying) == 0 {
			if ops := fanOut(m, all, main); len(ops) > 0 {
				steps = append(steps, autoStep(rel, ops))
			}
			break
		}
		for _, wt := range c.modifying {
			if same(m.Path(), filepath.Join(wt, rel)) {
				steps = append(steps, autoStep(rel, []Op{cp(main, wt, rel)})) // aligns mtime
				continue
			}
			st := mainVsWT(rel, "conflict: "+rel+" modified in main and "+filepath.Base(wt), wt,
				[]Op{cp(main, wt, rel)},
				append([]Op{cp(wt, main, rel)}, fanOut(m, all, wt, main)...),
				[]Op{conflictCopy(wt, main, rel, filepath.Base(wt))})
			st.Choice.RecheckSame = &[2]Loc{m, {wt, rel}}
			steps = append(steps, st)
		}
		// Untouched and missing worktrees hold nothing new, so they take main's final version;
		// otherwise the rebuilt manifest would hide that they are stale.
		var ops []Op
		for _, w := range append(slices.Clone(c.untouched), c.missing...) {
			ops = append(ops, cp(main, w, rel))
		}
		if len(ops) > 0 {
			steps = append(steps, autoStep(rel, ops))
		}
	default: // R5, git-wt-sync:553-611
		if len(c.modifying) > 0 {
			steps = fromModifying(main, all, rel, c.modifying, same, false)
		} else if len(c.missing) > 0 {
			var ops []Op
			for _, w := range c.missing {
				ops = append(ops, cp(main, w, rel))
			}
			steps = append(steps, autoStep(rel, ops))
		}
	}
	return steps
}

// BuildPlan applies the git-wt-sync rules (first match wins) to every file. same compares two
// absolute paths by content and is only called where the Bash runs `diff -q`.
func BuildPlan(main string, others []string, files []FileState, same func(a, b string) bool) Plan {
	p := Plan{Main: main}
	for _, f := range files {
		p.Steps = append(p.Steps, planFile(main, others, f, same)...)
	}
	return p
}

// CheckOutOfSync returns the `git wt sync --check` report lines (git-wt-sync:353-380) without colours.
func CheckOutOfSync(main string, others []string, files []FileState, same func(a, b string) bool) []string {
	var lines []string
	for _, f := range files {
		c := classify(main, others, f, same)
		if !(c.mainDeleted || len(c.deleting) > 0 || !c.existsMain || c.mainModified || len(c.modifying) > 0 || len(c.missing) > 0) {
			continue
		}
		lines = append(lines, "Out of sync: "+f.Rel)
		if c.mainDeleted {
			lines = append(lines, "    - Deleted in main worktree")
		}
		if len(c.deleting) > 0 {
			lines = append(lines, "    - Deleted in: "+names(c.deleting, " "))
		}
		if !c.existsMain {
			lines = append(lines, "    - New file in: "+names(c.modifying, " "))
		}
		if c.mainModified {
			lines = append(lines, "    - Modified in main worktree")
		}
		if len(c.modifying) > 0 && c.existsMain {
			lines = append(lines, "    - Modified in: "+names(c.modifying, " "))
		}
		if len(c.missing) > 0 {
			lines = append(lines, "    - Missing in: "+names(c.missing, " "))
		}
	}
	return lines
}
