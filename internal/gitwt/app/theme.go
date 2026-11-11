package wtapp

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/mukeshmk/devex-manager/internal/shared/config"
)

// Themed styles, set by SetTheme; init applies lazygit's defaults.
var (
	active, inactive, searching, hintStyle lipgloss.Style
	themeWarning                           string // shown in the status line when the app opens
)

func init() { SetTheme(config.DefaultTheme()) }

var (
	colourNames = map[string]string{"black": "0", "red": "1", "green": "2", "yellow": "3", "blue": "4", "magenta": "5", "cyan": "6", "white": "7"}
	hexColour   = regexp.MustCompile(`^#[0-9a-f]{6}$`)
)

// parseSpec reads a lazygit-style spec: one colour and any of bold, underline, reverse. bg makes the colour the background.
func parseSpec(spec string, bg bool) (lipgloss.Style, error) {
	st := lipgloss.NewStyle()
	colour := false
	for _, tok := range strings.Split(strings.ToLower(spec), ",") {
		tok = strings.TrimSpace(tok)
		c, named := colourNames[tok]
		n, err := strconv.Atoi(tok)
		switch {
		case tok == "bold":
			st = st.Bold(true)
		case tok == "underline":
			st = st.Underline(true)
		case tok == "reverse":
			st = st.Reverse(true)
		case !colour && tok == "default":
			colour = true
		case !colour && (named || hexColour.MatchString(tok) || err == nil && n >= 0 && n <= 255):
			if !named {
				c = tok
			}
			if bg {
				st = st.Background(lipgloss.Color(c))
			} else {
				st = st.Foreground(lipgloss.Color(c))
			}
			colour = true
		default:
			return lipgloss.NewStyle(), fmt.Errorf("unknown colour %q", tok)
		}
	}
	return st, nil
}

// SetTheme applies t to the app's styles; a bad value falls back to its default, and the first one is returned as a warning.
func SetTheme(t config.Theme) (warning string) {
	d := config.DefaultTheme()
	parse := func(key, spec, def string, bg bool) lipgloss.Style {
		st, err := parseSpec(spec, bg)
		if err != nil {
			if warning == "" {
				warning = fmt.Sprintf("theme: %v in %s, using default", err, key)
			}
			st, _ = parseSpec(def, bg)
		}
		return st
	}
	active = parse("active_border_color", t.ActiveBorder, d.ActiveBorder, false)
	inactive = parse("inactive_border_color", t.InactiveBorder, d.InactiveBorder, false)
	searching = parse("searching_border_color", t.SearchingBorder, d.SearchingBorder, false)
	hintStyle = parse("options_text_color", t.OptionsText, d.OptionsText, false)
	accent = active
	themeWarning = warning
	return warning
}
