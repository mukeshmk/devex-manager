package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mukeshmk/devex-manager/internal/ctx"
	"github.com/mukeshmk/devex-manager/internal/gitx"
	"github.com/mukeshmk/devex-manager/internal/ui"
)

const ctxHelp = `DevEx Manager (dx ctx)
Usage: dx ctx [command] [arguments]

Branch-specific scratchpad and todo checklists.

Commands:
  (none/show)       - Show notes and todo checklist for current branch
  edit              - Open context file in your default editor
  add <task>        - Add a new task (shortcut for todo add)
  done <index>      - Mark task as completed (shortcut for todo done)
  undo <index>      - Mark task as incomplete (shortcut for todo undo)
  rm <index>        - Delete a task (shortcut for todo rm)
  clean             - Interactively delete notes for deleted local branches

Todo Sub-commands:
  todo add <task>   - Add a new checklist task
  todo list         - List all tasks
  todo done <idx>   - Check off a task
  todo undo <idx>   - Uncheck a task
  todo rm <idx>     - Delete a task
  todo clear        - Purge all checked/completed tasks

Data is stored locally under .git/info/devex/contexts/ and is never committed.
`

// ctxEnv is the shared per-invocation setup (dx-ctx-helper.sh).
type ctxEnv struct {
	out, errw io.Writer
	in        *os.File
	repo      *gitx.Repo
	branch    string
	path      string
	co        palette
}

func (e *ctxEnv) fail(msg string) error {
	fmt.Fprintln(e.errw, colours(e.errw).red.Render(msg))
	return errExit1
}

func (e *ctxEnv) read() (string, error) {
	b, err := os.ReadFile(e.path)
	if err != nil {
		return "", e.fail(err.Error())
	}
	return string(b), nil
}

// mutate applies fn to the file content, writes it atomically, then prints the success line.
func (e *ctxEnv) mutate(fn func(string) (string, string, error)) error {
	c, err := e.read()
	if err != nil {
		return err
	}
	nc, msg, err := fn(c)
	if err != nil {
		return e.fail(err.Error())
	}
	if err := ctx.Write(e.path, nc); err != nil {
		return e.fail(err.Error())
	}
	fmt.Fprintln(e.out, e.co.green.Render(msg))
	return nil
}

func ctxSetup(c *cobra.Command) (*ctxEnv, error) {
	e := &ctxEnv{out: c.OutOrStdout(), errw: c.ErrOrStderr()}
	e.in, _ = c.InOrStdin().(*os.File)
	e.co = colours(e.out)
	cwd, err := os.Getwd()
	if err != nil {
		return nil, e.fail(err.Error())
	}
	if _, err := gitx.Run(cwd, "rev-parse", "--is-inside-work-tree"); err != nil {
		return nil, e.fail("Error: Not a git repository (or any of the parent directories)")
	}
	if e.repo, err = gitx.Open(cwd); err != nil {
		return nil, e.fail(err.Error())
	}
	e.branch, _ = gitx.Run(cwd, "rev-parse", "--abbrev-ref", "HEAD")
	if e.branch == "" || e.branch == "HEAD" {
		return nil, e.fail("Error: Not currently on a branch (detached HEAD)")
	}
	e.path = ctx.Path(e.repo.CommonDir, e.branch)
	if err := ctx.EnsureFile(e.path, e.branch); err != nil {
		return nil, e.fail(err.Error())
	}
	return e, nil
}

// ctxCmd wraps run with setup; flag parsing is off so tasks like "-x" are taken literally, as in Bash.
func ctxCmd(use string, aliases []string, run func(*ctxEnv, []string) error) *cobra.Command {
	return &cobra.Command{Use: use, Aliases: aliases, DisableFlagParsing: true, Args: cobra.ArbitraryArgs,
		RunE: func(c *cobra.Command, args []string) error {
			if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
				fmt.Fprint(c.OutOrStdout(), ctxHelp)
				return nil
			}
			e, err := ctxSetup(c)
			if err != nil {
				return err
			}
			return run(e, args)
		}}
}

func (e *ctxEnv) show(_ []string) error {
	c, err := e.read()
	if err != nil {
		return err
	}
	co := e.co
	fmt.Fprintln(e.out, co.blue.Bold(true).Render("Context: "+e.branch))
	fmt.Fprintln(e.out)
	todos := ctx.ParseTodos(c)
	// shortcut: Todos always print before Notes, whatever order the file has
	if ctx.HasTodos(c) {
		fmt.Fprintln(e.out, co.blue.UnsetForeground().Bold(true).Render("Todos:"))
	}
	for _, t := range todos {
		if t.Done {
			fmt.Fprintln(e.out, co.green.Render(fmt.Sprintf("  [%d] [✓] %s", t.Index, t.Text)))
		} else {
			fmt.Fprintln(e.out, co.yellow.Render(fmt.Sprintf("  [%d] [ ] %s", t.Index, t.Text)))
		}
	}
	if ctx.HasNotes(c) {
		fmt.Fprintln(e.out, "\n"+co.blue.UnsetForeground().Bold(true).Render("Notes:"))
	}
	for _, l := range ctx.Notes(c) {
		fmt.Fprintln(e.out, "  "+l)
	}
	if len(todos) == 0 {
		fmt.Fprintln(e.out, co.yellow.Render("  No tasks. Add one with 'dx ctx todo add <task>'"))
	}
	return nil
}

func (e *ctxEnv) add(args []string) error {
	task := strings.Join(args, " ")
	if task == "" {
		return e.fail("Error: Task description cannot be empty.")
	}
	return e.mutate(func(c string) (string, string, error) {
		return ctx.AddTodo(c, task), "✓ Added task: " + task, nil
	})
}

// byIndex builds done/undo/rm: missing arg and non-numeric or unknown indexes print the Bash errors.
func byIndex(verb string, apply func(c string, idx int) (string, error), msg string) func(*ctxEnv, []string) error {
	return func(e *ctxEnv, args []string) error {
		if len(args) == 0 || args[0] == "" {
			return e.fail("Error: Specify a todo index to " + verb + ".")
		}
		idx, err := strconv.Atoi(args[0])
		if err != nil {
			idx = -1
		}
		return e.mutate(func(c string) (string, string, error) {
			nc, err := apply(c, idx)
			if err != nil { // keep the user's text for non-numeric input
				return "", "", fmt.Errorf("Error: Todo index %s not found.", args[0])
			}
			return nc, fmt.Sprintf(msg, args[0]), nil
		})
	}
}

func (e *ctxEnv) clear(_ []string) error {
	c, err := e.read()
	if err != nil {
		return err
	}
	if !ctx.HasTodos(c) {
		fmt.Fprintln(e.out, e.co.yellow.Render("No todos section found."))
		return nil
	}
	return e.mutate(func(c string) (string, string, error) {
		nc, n := ctx.ClearDone(c)
		return nc, fmt.Sprintf("✓ Cleared %d completed tasks.", n), nil
	})
}

func (e *ctxEnv) edit(_ []string) error {
	argv := strings.Fields(os.Getenv("EDITOR"))
	if len(argv) == 0 {
		for _, n := range []string{"nano", "vim", "vi"} {
			if p, err := exec.LookPath(n); err == nil {
				argv = []string{p}
				break
			}
		}
	}
	if len(argv) == 0 {
		return e.fail("Error: No text editor found. Please set $EDITOR environment variable.")
	}
	cmd := exec.Command(argv[0], append(argv[1:], e.path)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = e.in, e.out, e.errw
	if e.in == nil {
		cmd.Stdin = os.Stdin
	}
	if err := cmd.Run(); err != nil {
		return e.fail("Error: " + err.Error())
	}
	return nil
}

func (e *ctxEnv) clean(_ []string) error {
	cd := filepath.Dir(e.path)
	if _, err := os.Stat(cd); err != nil {
		fmt.Fprintln(e.out, e.co.yellow.Render("No contexts folder found."))
		return nil
	}
	cwd, _ := os.Getwd()
	exists := func(b string) bool {
		_, err := gitx.Run(cwd, "show-ref", "--verify", "--quiet", "refs/heads/"+b)
		return err == nil
	}
	orphans := ctx.Orphans(e.repo.CommonDir, exists)
	if len(orphans) == 0 {
		fmt.Fprintln(e.out, e.co.green.Render("No orphan contexts found. System is clean!"))
		return nil
	}
	fmt.Fprintln(e.out, e.co.yellow.Render(fmt.Sprintf("Found %d orphan contexts (branches no longer exist):", len(orphans))))
	for _, b := range orphans {
		fmt.Fprintf(e.out, "  - %s (file: %s)\n", b, filepath.Base(ctx.Path(e.repo.CommonDir, b)))
	}
	fmt.Fprintln(e.out)
	if !ui.NewPrompter(e.in, e.out).Confirm("Would you like to delete these orphan contexts?", false) {
		fmt.Fprintln(e.out, "Cleanup cancelled.")
		return nil
	}
	for _, b := range orphans {
		if err := os.Remove(ctx.Path(e.repo.CommonDir, b)); err != nil {
			return e.fail(err.Error())
		}
		fmt.Fprintln(e.out, "  "+e.co.green.Render("✓ Deleted context file for "+b))
	}
	fmt.Fprintln(e.out, e.co.green.Render("Cleanup complete!"))
	return nil
}

func init() {
	done := byIndex("check", func(c string, i int) (string, error) { return ctx.SetDone(c, i, true) }, "✓ Checked task %s.")
	undo := byIndex("uncheck", func(c string, i int) (string, error) { return ctx.SetDone(c, i, false) }, "✓ Unchecked task %s.")
	rm := byIndex("delete", ctx.RemoveTodo, "✓ Deleted task %s.")

	cmds := []*cobra.Command{
		ctxCmd("show", nil, (*ctxEnv).show),
		ctxCmd("edit", nil, (*ctxEnv).edit),
		ctxCmd("add <task>", nil, (*ctxEnv).add),
		ctxCmd("done <index>", nil, done),
		ctxCmd("undo <index>", nil, undo),
		ctxCmd("rm <index>", nil, rm),
		ctxCmd("clean", nil, (*ctxEnv).clean),
		{Use: "help", DisableFlagParsing: true, RunE: func(c *cobra.Command, _ []string) error {
			fmt.Fprint(c.OutOrStdout(), ctxHelp)
			return nil
		}},
	}
	todo := &cobra.Command{Use: "todo", DisableFlagParsing: true, Args: cobra.ArbitraryArgs,
		RunE: func(c *cobra.Command, args []string) error {
			if len(args) > 0 && args[0] != "-h" && args[0] != "--help" {
				fmt.Fprintln(c.OutOrStdout(), colours(c.OutOrStdout()).red.Render("Unknown todo sub-command: "+args[0]))
				fmt.Fprintln(c.OutOrStdout(), "Run 'dx ctx help' for usage.")
				return errExit1
			}
			if len(args) > 0 {
				fmt.Fprint(c.OutOrStdout(), ctxHelp)
				return nil
			}
			e, err := ctxSetup(c)
			if err != nil {
				return err
			}
			return e.show(nil)
		}}
	for _, s := range []*cobra.Command{
		ctxCmd("list", nil, (*ctxEnv).show),
		ctxCmd("add <task>", nil, (*ctxEnv).add),
		ctxCmd("done <idx>", []string{"check"}, done),
		ctxCmd("undo <idx>", []string{"uncheck"}, undo),
		ctxCmd("rm <idx>", []string{"delete"}, rm),
		ctxCmd("clear", nil, (*ctxEnv).clear),
	} {
		todo.AddCommand(s)
	}
	cmds = append(cmds, todo)
	for _, c := range cmds {
		registerDx("ctx", c)
	}
	// Bare "dx ctx" shows; any other first word is taken as a task to add.
	dxGroupRun["ctx"] = func(c *cobra.Command, args []string) error {
		switch {
		case len(args) == 0 || args[0] == "": // Bash: ${1:-show}
			e, err := ctxSetup(c)
			if err != nil {
				return err
			}
			return e.show(nil)
		case args[0] == "-h" || args[0] == "--help":
			fmt.Fprint(c.OutOrStdout(), ctxHelp)
			return nil
		}
		e, err := ctxSetup(c)
		if err != nil {
			return err
		}
		return e.add(args)
	}
}
