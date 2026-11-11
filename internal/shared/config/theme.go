package config

import "path/filepath"

// Theme holds the raw [theme] colour specs from ~/.devex.conf, as written (e.g. "green, bold").
type Theme struct {
	ActiveBorder, InactiveBorder, SearchingBorder, OptionsText, SelectedLineBg string
}

// DefaultTheme is lazygit's default gui.theme.
func DefaultTheme() Theme {
	return Theme{
		ActiveBorder:    "green, bold",
		InactiveBorder:  "default",
		SearchingBorder: "cyan, bold",
		OptionsText:     "blue",
		SelectedLineBg:  "blue",
	}
}

// LoadTheme reads [theme] from <home>/.devex.conf over DefaultTheme; a missing file is not an error.
func LoadTheme(home string) (Theme, error) {
	t := DefaultTheme()
	ini, err := readINI(filepath.Join(home, ".devex.conf"))
	if err != nil {
		return t, err
	}
	for k, f := range map[string]*string{
		"active_border_color":    &t.ActiveBorder,
		"inactive_border_color":  &t.InactiveBorder,
		"searching_border_color": &t.SearchingBorder,
		"options_text_color":     &t.OptionsText,
		"selected_line_bg_color": &t.SelectedLineBg,
	} {
		if v := ini["theme"][k]; v != "" {
			*f = v
		}
	}
	return t, nil
}
