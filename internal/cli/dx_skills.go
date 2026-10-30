package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mukeshmk/devex-manager/internal/config"
	"github.com/mukeshmk/devex-manager/internal/skills"
	"github.com/mukeshmk/devex-manager/internal/skillsapp"
	"github.com/mukeshmk/devex-manager/internal/ui"
)

// Overridden in tests so the real ~/.devex.conf and environment are never read.
var (
	skillsEnv  = os.Getenv
	skillsHome = os.UserHomeDir
)

const skillsUsage = "Usage: dx skills [list|sync|diff|edit|rm] [arguments]"

func init() {
	// No args: the grid app on a TTY, else list (non-TTY never opens a full-screen app).
	dxGroupRun["skills"] = func(c *cobra.Command, args []string) error {
		if len(args) > 0 {
			fmt.Fprintln(c.ErrOrStderr(), colours(c.ErrOrStderr()).red.Render(fmt.Sprintf("Error: Unknown command '%s'", args[0])))
			fmt.Fprintln(c.ErrOrStderr(), skillsUsage)
			return errExit1
		}
		return withSkills(c, nil, "", func(s *skillsRun) error {
			if f, ok := s.out.(*os.File); ok && ui.IsTTY(f) && ui.IsTTY(s.in) {
				if err := skillsapp.Run(skillsapp.New(s.cfg)); err != nil {
					return s.fail(err)
				}
				return nil
			}
			return s.list()
		})
	}
	registerDx("skills", &cobra.Command{Use: "list", Short: "List skills and their newest copy", Args: cobra.ArbitraryArgs,
		RunE: func(c *cobra.Command, _ []string) error { return withSkills(c, nil, "", (*skillsRun).list) }})
	var yes bool
	sync := &cobra.Command{Use: "sync", Short: "Copy the newest version of each skill everywhere", Args: cobra.ArbitraryArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			return withSkills(c, nil, "", func(s *skillsRun) error { return s.sync(yes) })
		}}
	sync.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation")
	registerDx("skills", sync)
	for _, n := range []struct {
		name, short string
		fn          func(*skillsRun, string) error
	}{
		{"diff", "Show differences between copies of a skill", (*skillsRun).diff},
		{"edit", "Edit a skill in master, then sync", (*skillsRun).edit},
		{"rm", "Delete a skill everywhere (with a backup)", (*skillsRun).rm},
	} {
		registerDx("skills", &cobra.Command{Use: n.name + " <skill>", Short: n.short, Args: cobra.ArbitraryArgs,
			RunE: func(c *cobra.Command, args []string) error {
				return withSkills(c, args, n.name, func(s *skillsRun) error { return n.fn(s, args[0]) })
			}})
	}
}

type skillsRun struct {
	in        *os.File
	out, errw io.Writer
	co, ce    palette
	cfg       config.SkillsConfig
	pr        ui.Prompter
}

// withSkills loads the config once per invocation (so the warning prints at most once) and runs fn.
// A non-empty needArg means fn needs args[0].
func withSkills(c *cobra.Command, args []string, needArg string, fn func(*skillsRun) error) error {
	in, _ := c.InOrStdin().(*os.File)
	if in == nil {
		in = os.Stdin
	}
	s := &skillsRun{in: in, out: c.OutOrStdout(), errw: c.ErrOrStderr()}
	s.co, s.ce = colours(s.out), colours(s.errw)
	if needArg != "" && (len(args) == 0 || args[0] == "") {
		fmt.Fprintln(s.errw, s.ce.red.Render("Error: Skill name argument required."))
		fmt.Fprintf(s.errw, "Usage: dx skills %s <skill_name>\n", needArg)
		return errExit1
	}
	if needArg != "" {
		if err := skills.ValidName(args[0]); err != nil {
			return s.fail(err)
		}
	}
	home, err := skillsHome()
	if err != nil {
		return s.fail(err)
	}
	if s.cfg, err = config.LoadSkills(home, skillsEnv); err != nil {
		return s.fail(err)
	}
	if s.cfg.Warning != "" {
		for _, l := range strings.Split(s.cfg.Warning, "\n") {
			fmt.Fprintln(s.errw, s.ce.yellow.Render(l))
		}
		fmt.Fprintln(s.errw)
	}
	s.pr = ui.NewPrompter(in, s.out) // one prompter, so edit + sync share buffered stdin
	return fn(s)
}

func (s *skillsRun) fail(err error) error {
	fmt.Fprintln(s.errw, s.ce.red.Render(err.Error()))
	return errExit1
}

func (s *skillsRun) list() error {
	sk, err := skills.Scan(s.cfg)
	if err != nil {
		return s.fail(err)
	}
	var tools []string
	for _, t := range s.cfg.Tools {
		tools = append(tools, t.Name)
	}
	fmt.Fprintln(s.out, s.co.blue.Render("Master Directory: ")+s.cfg.MasterDir)
	fmt.Fprintln(s.out, s.co.blue.Render("Active Agents:    ")+strings.Join(tools, ","))
	fmt.Fprintln(s.out)
	rows := make([][]string, 0, len(sk))
	for _, k := range sk {
		newest, date := "None", "Never"
		for _, c := range k.Copies {
			if c.Location == k.Newest {
				newest, date = c.Location, time.Unix(c.Mtime, 0).Format("2006-01-02 15:04:05")
			}
		}
		rows = append(rows, []string{k.Name, k.Description, newest, date})
	}
	f, _ := s.out.(*os.File)
	fmt.Fprintln(s.out, ui.Table([]ui.Column{{Title: "Skill", Min: 5}, {Title: "Description", Min: 11, Flex: true},
		{Title: "Newest", Min: 6}, {Title: "Last Modified", Min: 13}}, rows, ui.Width(f)))
	return nil
}

func (s *skillsRun) sync(yes bool) error {
	sk, err := skills.Scan(s.cfg)
	if err != nil {
		return s.fail(err)
	}
	ups := skills.Plan(s.cfg, sk, skills.SameDir)
	if len(ups) == 0 {
		fmt.Fprintln(s.out, s.co.green.Render("All skills are already synchronized!"))
		return nil
	}
	fmt.Fprintln(s.out, s.co.blue.Render("The following synchronizations are pending:"))
	for i := 0; i < len(ups); { // ups are grouped by skill (Scan sorts, Plan keeps order)
		j, dests := i, []string{}
		for ; j < len(ups) && ups[j].Skill == ups[i].Skill; j++ {
			dests = append(dests, ups[j].To)
		}
		fmt.Fprintf(s.out, "  - %s  Copying from %s  -->  %s\n", s.co.yellow.Render("["+ups[i].Skill+"]"),
			s.co.green.Render(ups[i].From), s.co.blue.Render(strings.Join(dests, ",")))
		i = j
	}
	fmt.Fprintln(s.out)
	if !yes && !s.pr.Confirm("Proceed with synchronization?", false) {
		fmt.Fprintln(s.out, s.co.red.Render("Synchronization cancelled."))
		return errExit1
	}
	fmt.Fprintln(s.out)
	root, done, err := skills.ApplyUpdates(ups, s.cfg.MasterDir, time.Now())
	for _, u := range ups[:done] {
		fmt.Fprintf(s.out, "  [sync] %s: %s -> %s... Done\n", u.Skill, u.From, u.To)
	}
	if err != nil {
		fmt.Fprintln(s.errw, s.ce.red.Render(err.Error()))
		if root != "" {
			fmt.Fprintln(s.errw, "Backups stored in: "+root)
		}
		return errExit1
	}
	fmt.Fprintln(s.out, "\n"+s.co.green.Render("✓ Synchronization complete!"))
	if root != "" { // nothing existed to back up: no backup dir was made
		fmt.Fprintln(s.out, "Backups stored in: "+s.co.blue.Render(root))
	}
	return nil
}

func (s *skillsRun) locations(name, notFound string) ([]skills.Copy, []string, error) {
	locs := skills.Locations(s.cfg, name)
	if len(locs) == 0 {
		return nil, nil, s.fail(fmt.Errorf(notFound, name))
	}
	names := make([]string, len(locs))
	for i, c := range locs {
		names[i] = c.Location
	}
	return locs, names, nil
}

func (s *skillsRun) diff(name string) error {
	locs, names, err := s.locations(name, "Error: Skill '%s' not found in any active directory.")
	if err != nil {
		return err
	}
	if len(locs) == 1 {
		fmt.Fprintf(s.out, "Skill '%s' only exists in %s (no other versions to diff against).\n", name, names[0])
		return nil
	}
	found := false
	for _, c := range locs[1:] {
		if skills.SameDir(locs[0].Dir, c.Dir) {
			continue
		}
		found = true
		fmt.Fprintln(s.out, s.co.blue.Render(fmt.Sprintf("=== Diff: %s vs %s ===", names[0], c.Location)))
		d, err := skills.Diff(locs[0].Dir, c.Dir)
		if err != nil {
			return s.fail(err)
		}
		fmt.Fprint(s.out, d)
		fmt.Fprintln(s.out)
	}
	if !found {
		fmt.Fprintln(s.out, s.co.green.Render(fmt.Sprintf("Skill '%s' is in sync across all active targets (%s).", name, strings.Join(names, " "))))
	}
	return nil
}

func (s *skillsRun) rm(name string) error {
	_, names, err := s.locations(name, "Error: Skill '%s' not found.")
	if err != nil {
		return err
	}
	fmt.Fprintln(s.out, s.co.red.Render(fmt.Sprintf("WARNING: You are about to delete skill '%s' from: %s", name, strings.Join(names, " "))))
	if !s.pr.Confirm("Are you sure you want to proceed?", false) {
		fmt.Fprintln(s.out, "Cancelled.")
		return nil
	}
	root, err := skills.Remove(s.cfg, name, time.Now())
	if err != nil {
		fmt.Fprintln(s.errw, s.ce.red.Render(err.Error()))
		if root != "" {
			fmt.Fprintln(s.errw, "Backup stored in: "+root)
		}
		return errExit1
	}
	for _, n := range names {
		fmt.Fprintf(s.out, "  [delete] %s from %s... Done\n", name, n)
	}
	fmt.Fprintln(s.out, "\n"+s.co.green.Render(fmt.Sprintf("✓ Skill '%s' deleted successfully.", name)))
	fmt.Fprintln(s.out, "Backup stored in: "+s.co.blue.Render(filepath.Join(root, "deleted")))
	return nil
}

func (s *skillsRun) edit(name string) error {
	dir := filepath.Join(s.cfg.MasterDir, name)
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		fmt.Fprintln(s.out, s.co.yellow.Render(fmt.Sprintf("Skill '%s' does not exist in master repository (%s).", name, s.cfg.MasterDir)))
		if !s.pr.Confirm("Would you like to create it?", false) {
			fmt.Fprintln(s.out, "Cancelled.")
			return nil
		}
		if err := skills.CreateTemplate(s.cfg.MasterDir, name); err != nil {
			return s.fail(err)
		}
		fmt.Fprintln(s.out, s.co.green.Render("✓ Created skill folder and default SKILL.md template."))
	}
	file := skills.SkillFile(dir)
	if file == "" {
		file = filepath.Join(dir, "SKILL.md")
		if err := os.WriteFile(file, nil, 0o644); err != nil {
			return s.fail(err)
		}
	}
	editor := strings.Fields(skillsEnv("EDITOR")) // unquoted ${EDITOR:-nano}: word-split
	if len(editor) == 0 {
		editor = []string{"nano"}
	}
	cmd := exec.Command(editor[0], append(editor[1:], file)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = s.in, s.out, s.errw
	if err := cmd.Run(); err != nil {
		return s.fail(fmt.Errorf("Error: editor %s failed: %w", editor[0], err))
	}
	fmt.Fprintln(s.out, s.co.green.Render("Changes saved."))
	return s.sync(false)
}
