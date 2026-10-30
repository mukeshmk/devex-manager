package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/mukeshmk/devex-manager/internal/nb"
	"github.com/mukeshmk/devex-manager/internal/shared/ui"
)

const nbUsage = `Usage: dx nb <command> [arguments]

Available commands:
  strip   - Clear outputs/metadata (bulk support)
  diff    - Human-readable notebook diffs
  kernel  - Register .venv as Jupyter kernel
  list    - Recursive discovery & metadata reporting

`

func init() {
	dxGroupRun["nb"] = func(c *cobra.Command, _ []string) error {
		st := lipgloss.NewRenderer(c.OutOrStdout()).NewStyle().Foreground(lipgloss.Color("4"))
		fmt.Fprintln(c.OutOrStdout(), st.Render("DevEx Manager (dx nb)"))
		io.WriteString(c.OutOrStdout(), nbUsage)
		return errExit1
	}
	// DisableFlagParsing passes flags like --stat straight through to the external tool.
	mk := func(use, short string, f func(c *cobra.Command, args []string) error) {
		registerDx("nb", &cobra.Command{Use: use, Short: short, DisableFlagParsing: true, RunE: f})
	}
	mk("strip", "Clear outputs/metadata from notebooks", nbStrip)
	mk("diff", "Human-readable notebook diffs", nbDiff)
	mk("kernel", "Register .venv as Jupyter kernel", nbKernel)
	mk("list", "List notebooks with metadata", nbList)
}

func nbStyles(w io.Writer) (blue, green, yellow, red lipgloss.Style) {
	r := lipgloss.NewRenderer(w)
	s := func(c string) lipgloss.Style { return r.NewStyle().Foreground(lipgloss.Color(c)) }
	return s("4"), s("2"), s("3"), s("1")
}

// nbExec runs name with stdio on the Run streams; a non-zero exit becomes errExit1 after propagating output.
func nbExec(c *cobra.Command, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = c.InOrStdin(), c.OutOrStdout(), c.ErrOrStderr()
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return exitCode(ee.ExitCode())
		}
		return err
	}
	return nil
}

func nbStrip(c *cobra.Command, args []string) error {
	out, errw := c.OutOrStdout(), c.ErrOrStderr()
	blue, green, _, red := nbStyles(out)
	if len(args) == 0 {
		io.WriteString(out, "Usage: dx nb strip <notebook1.ipynb> [notebook2.ipynb ...]\nExample: dx nb strip *.ipynb\n")
		return errExit1
	}
	var cmd []string
	if _, err := exec.LookPath("nbstripout"); err == nil {
		fmt.Fprintln(out, blue.Render(fmt.Sprintf("Stripping output from %d notebook(s) using nbstripout...", len(args))))
		cmd = []string{"nbstripout"}
	} else if _, err := exec.LookPath("jupyter"); err == nil {
		fmt.Fprintln(out, blue.Render(fmt.Sprintf("Stripping output from %d notebook(s) using jupyter nbconvert...", len(args))))
		cmd = []string{"jupyter", "nbconvert", "--clear-output", "--inplace"}
	} else {
		fmt.Fprintln(errw, red.Render("Error: Neither 'nbstripout' nor 'jupyter' (nbconvert) is installed."))
		io.WriteString(errw, "Please install one of them to use this feature:\n  pip install nbstripout\n  pip install nbconvert\n")
		return errExit1
	}
	if err := nbExec(c, cmd[0], append(cmd[1:], args...)...); err != nil {
		return err
	}
	fmt.Fprintln(out, green.Render("✓ Done."))
	return nil
}

func nbDiff(c *cobra.Command, args []string) error {
	out, errw := c.OutOrStdout(), c.ErrOrStderr()
	blue, _, yellow, _ := nbStyles(errw)
	if len(args) == 0 {
		io.WriteString(out, "Usage: dx nb diff <notebook1.ipynb> [arguments to git diff]\n")
		return errExit1
	}
	if _, err := exec.LookPath("nbdiff"); err == nil {
		return nbExec(c, "nbdiff", args...)
	}
	fmt.Fprintln(errw, yellow.Render("Warning: 'nbdime' is not installed. Falling back to raw JSON diff."))
	fmt.Fprintln(errw, "To get human-readable notebook diffs, run: "+blue.Render("pip install nbdime"))
	fmt.Fprintln(out)
	return nbExec(c, "git", append([]string{"diff"}, args...)...)
}

func nbKernel(c *cobra.Command, _ []string) error {
	out, errw := c.OutOrStdout(), c.ErrOrStderr()
	blue, green, _, red := nbStyles(out)
	if st, err := os.Stat(".venv"); err != nil || !st.IsDir() {
		fmt.Fprintln(errw, red.Render("Error: No .venv found in the current directory."))
		return errExit1
	}
	wd, _ := os.Getwd()
	name := filepath.Base(wd)
	py := "python"
	if _, err := os.Stat(".venv/bin/python"); err == nil {
		py = "./.venv/bin/python"
	}
	fmt.Fprintln(out, blue.Render("Registering kernel for worktree: "+name+"..."))
	if exec.Command(py, "-m", "ipykernel", "--version").Run() != nil {
		fmt.Fprintln(errw, red.Render("Error: 'ipykernel' is not installed in the .venv."))
		io.WriteString(errw, "Please install it (e.g., uv add ipykernel) and try again.\n")
		return errExit1
	}
	if err := nbExec(c, py, "-m", "ipykernel", "install", "--user", "--name", name, "--display-name", "DevEx: "+name); err != nil {
		return err
	}
	fmt.Fprintln(out, green.Render("✓ Kernel '"+name+"' registered successfully."))
	return nil
}

func humanSize(n int64) string {
	f, u := float64(n), "BKMGT"
	i := 0
	for f >= 1024 && i < 4 {
		f /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%dB", n)
	}
	return fmt.Sprintf("%.1f%c", f, u[i])
}

func nbList(c *cobra.Command, _ []string) error {
	out := c.OutOrStdout()
	f, _ := out.(*os.File)
	_, green, yellow, red := nbStyles(out)
	bold := lipgloss.NewRenderer(out).NewStyle().Bold(true)
	paths := nb.Find(".")
	if len(paths) == 0 {
		fmt.Fprintln(out, yellow.Render("No notebooks found in current directory."))
		return nil
	}
	tty := ui.IsTTY(f)
	width := 100
	if tty {
		width = ui.Width(f)
	}
	var rows [][]string
	var total, cells, code, md, dirty, clean int
	for _, p := range paths {
		i := nb.Inspect(p)
		total++
		status, cs := "Error (JSON)", "?"
		if i.Err == nil {
			cs = fmt.Sprintf("%d (%d/%d)", i.Cells, i.Code, i.Markdown)
			cells, code, md = cells+i.Cells, code+i.Code, md+i.Markdown
			if i.HasOutput {
				status, dirty = "● Dirty", dirty+1
			} else {
				status, clean = "○ Stripped", clean+1
			}
		}
		if tty {
			switch status {
			case "● Dirty":
				status = yellow.Render(status)
			case "○ Stripped":
				status = green.Render(status)
			default:
				status = red.Render(status)
			}
		}
		rows = append(rows, []string{p, humanSize(i.Size), cs, status, i.Modified.Format("2006-01-02 15:04")})
	}
	fmt.Fprintln(out, bold.Foreground(lipgloss.Color("4")).Render("Project Notebooks Status"))
	fmt.Fprintln(out, ui.Table([]ui.Column{{Title: "Notebook Path", Min: 20, Flex: true}, {Title: "Size"}, {Title: "Cells(C/M)"}, {Title: "Status"}, {Title: "Last Modified"}}, rows, width))
	fmt.Fprintln(out, bold.Render("Summary:"))
	fmt.Fprintf(out, "  Total Notebooks: %s\n", bold.Render(fmt.Sprint(total)))
	fmt.Fprintf(out, "  Total Cells:     %s (%d Code, %d Markdown)\n", bold.Render(fmt.Sprint(cells)), code, md)
	fmt.Fprintf(out, "  Status:          %s, %s\n", green.Render(fmt.Sprintf("%d Stripped", clean)), yellow.Render(fmt.Sprintf("%d Dirty", dirty)))
	return nil
}
