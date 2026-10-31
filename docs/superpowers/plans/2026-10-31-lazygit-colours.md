# git wt app: lazygit colour scheme, configurable — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** By default, the `git wt` full-screen app uses lazygit's colour scheme:
- the focused border is green and bold
- other borders use the terminal's default colour
- key hints are blue
- the filter border is cyan
- popups have green borders
- the selected row gets a full-width blue highlight

Every one of these colours can be changed in a `[theme]` section of `~/.devex.conf`.

**Architecture:**
- `config.LoadTheme` reads `[theme]` from `~/.devex.conf` and returns raw colour specs, with lazygit's defaults filled in.
- `wtapp.SetTheme` turns those specs into lipgloss styles. It replaces the app's style variables and returns a warning for each value it doesn't understand.
- The CLI calls it before starting the app, and any warning shows in the app's status line.
- The new helper `ui.Highlight` pads the selected row to the panel's inner width and paints it, keeping the colours already inside the row.

**Tech Stack:** Go 1.27, lipgloss v1.1.0, `github.com/charmbracelet/x/ansi`, and `github.com/muesli/termenv` v0.16.0. termenv is already an indirect dependency; tests import it to force a colour profile, so `go mod tidy` makes it direct. No new modules.

**Spec:** This plan is the spec. The owner asked for "the same lazygit colour scheme" in `git wt` only, chose part 1 (colours) and part 2 (selected-row highlight) from the estimate on 2026-10-31, and then asked for the colours to be configurable through a config file.

The defaults come from `lazygit --config` (lazygit 0.66.2) `gui.theme`:
- `activeBorderColor: [green, bold]`
- `inactiveBorderColor: [default]`
- `searchingActiveBorderColor: [cyan, bold]`
- `optionsTextColor: [blue]`
- `selectedLineFgColor: [bold]`
- lazygit's built-in default background for the selected line in the focused view is blue

## Config format (exact)

`~/.devex.conf`:
```ini
[theme]
# Colours for the git wt app. Same names and defaults as lazygit's gui.theme.
# A value is a comma-separated list: one colour and any of bold, underline, reverse.
# Colours: default, black, red, green, yellow, blue, magenta, cyan, white, 0-255, or #rrggbb.
active_border_color = green, bold
inactive_border_color = default
searching_border_color = cyan, bold
options_text_color = blue
selected_line_bg_color = blue
```
- A missing file, a missing `[theme]` section or a missing key falls back to the defaults above.
- Matching is case-insensitive, and spaces are trimmed.
- In `selected_line_bg_color`, the colour sets the background. Attributes in that key are applied too. The selected row is always bold, matching lazygit's `selectedLineFgColor`.
- If a value can't be read, only that key falls back to its default. The app starts anyway, and the status line shows `theme: unknown colour "<token>" in <key>, using default` until the first key press. If several values are bad, the first warning is shown.
- Only `~/.devex.conf`. A repo's `.devex.conf` has no `[theme]`, because colours are personal. There are no `DX_*` env overrides.

## Global Constraints

- Only `git wt`, meaning `internal/gitwt/app` and `internal/cli/wt_status.go`, plus new code in `internal/shared/config` and `internal/shared/ui`. The `dx skills` grid (`internal/skills/app`) does not change.
- With no `[theme]` section, these exact values apply:
  - focused border: green `"2"`, bold
  - unfocused border: no colour
  - filtering Worktrees border: cyan `"6"`, bold
  - key-hint line: blue `"4"`
  - popup and confirm-modal border: the active border style
  - selected row in the focused panel or popup: background blue `"4"`, bold
  - selected row in an unfocused panel: bold only
- Colour names map to ANSI indexes, so they follow the terminal palette the way lazygit does: black 0, red 1, green 2, yellow 3, blue 4, magenta 5, cyan 6, white 7. `default` means no colour.
- The `◆` focus marker and the `▸` row marker stay, so focus and selection are still visible without colour. Existing tests rely on them.
- No rendered line may exceed the terminal width at any size of 60×12 or larger. `Terminal too small (need 60×12)` stays.
- Status-line colours (red errors, yellow `running…`) and the text colours inside panels (yellow `merged`, green `[✓]`, dim notes) are not themed and do not change.
- The installer (`install.sh`) and its `~/.devex.conf` template do not change. The README documents `[theme]`.
- Never run host `git` in this repo. Commit only via `docker run --rm -v "$PWD":/devex-manager -w /devex-manager personal-git:v3 bash -c '…'`. No Co-Authored-By line. Never push.
- Tests: `go vet ./... && go test ./...` and `go test -race ./internal/gitwt/app/ ./internal/shared/ui/ ./internal/shared/config/`.

## Review Focus

1. **A malformed `[theme]` value** (`blu`, `#12`, `300`, empty): the app must still start, use the default for that key, keep the other keys, and show one warning in the status line. It must never fail to open. Tests: `TestLoadThemeBadValues` and `TestSetThemeWarns` in Task 1.
2. **A coloured segment inside the selected row** (yellow `merged`, green `[✓]`): the highlight background must continue across the whole row after each inner style's reset. Test: `TestHighlightKeepsBackgroundAfterReset` in Task 2.
3. **A selected row longer than its panel, or with wide characters:** the highlighted row must be exactly the inner width and truncated with `…`. Tests: `TestHighlightWidth`, plus the existing `TestWidthEverySize` sweep, in Task 2.
4. **A terminal without colour, and tests without a TTY:** lipgloss renders plain text there. Focus and selection must still show through `◆` and `▸`, and every existing test must pass unchanged. New colour assertions force a colour profile and restore it, and restore the default theme with `t.Cleanup`.
5. **Focus moving while a row is selected:** only the focused panel's selected row gets the background. Other panels show their selected row in bold. Test: `TestSelectedRowHighlightFollowsFocus` in Task 2.

---

## File Structure

```
internal/shared/config/theme.go       Theme, DefaultTheme, LoadTheme (Task 1)
internal/shared/config/config_test.go + theme tests (Task 1)
internal/gitwt/app/theme.go           SetTheme, parseSpec, theme style vars (Task 1); selected-row styles (Task 2)
internal/gitwt/app/app.go             style block + edge() use the theme vars (Task 1)
internal/gitwt/app/layout.go          filter border, hint line (Task 1); row highlight (Task 2)
internal/gitwt/app/popup.go           popup border (Task 1); cursor-row highlight (Task 2)
internal/gitwt/app/theme_test.go      colour assertions with a forced colour profile (Tasks 1–2)
internal/cli/wt_status.go             LoadTheme + SetTheme before the app starts (Task 1)
internal/shared/ui/box.go             + Highlight (Task 2)
internal/shared/ui/box_test.go        Highlight tests (Task 2)
README.md                             [theme] section docs (Task 2)
```

### Task 1: Configurable lazygit border and hint colours

**Files:**
- Create: `internal/shared/config/theme.go`, `internal/gitwt/app/theme.go`, and `internal/gitwt/app/theme_test.go`.
- Modify:
  - `internal/shared/config/config_test.go`
  - `internal/gitwt/app/app.go:549-566`: the style block and `edge()`. Also the confirm-modal `ui.Box(… accent)` at about line 637.
  - `internal/gitwt/app/layout.go`: the Worktrees border while `m.filtering || m.filter != ""`, and the `footer()` hint style at about line 278.
  - `internal/gitwt/app/popup.go:236`
  - `internal/gitwt/app/app.go` `New`: take the startup warning into `statusMsg`. Use the smallest change, for example a package-level `themeWarning` string that SetTheme sets and New reads.
  - `internal/cli/wt_status.go`

**Interfaces:**
- Produces, in package config:
  - `type Theme struct { ActiveBorder, InactiveBorder, SearchingBorder, OptionsText, SelectedLineBg string }`. These are the raw values as written, for example `"green, bold"`.
  - `func DefaultTheme() Theme`, returning the five defaults from the Config format section verbatim.
  - `func LoadTheme(home string) (Theme, error)`. It reads `<home>/.devex.conf` with the existing `readINI` and starts from `DefaultTheme()`. Each key present and non-empty in `[theme]` replaces its field. A missing file is not an error. A file that can't be read is an error, which the caller ignores and treats as defaults.
- Produces, in package wtapp:
  - The style vars `active`, `inactive`, `searching` and `hintStyle`. `edge(on)` returns `active` or `inactive`, and `accent` (the `▸` marker) is set equal to `active`.
  - `func SetTheme(t config.Theme) (warning string)`. It parses each field with `parseSpec`. A field that fails falls back to `parseSpec` of the same field in `config.DefaultTheme()`. It sets the style vars and returns the first warning, or `""` when everything parsed.
  - `func parseSpec(spec string, bg bool) (lipgloss.Style, error)`. It accepts the tokens listed in Config format. When `bg` is set, the colour becomes the background. A second colour in one value is an error, and so is an unknown token. The error text is `unknown colour "<token>"`, which SetTheme wraps as `theme: unknown colour "<token>" in <key>, using default`, where `<key>` is the ini key name.
- The CLI side, in `wt_status.go`, before `wtapp.New`: `home, _ := os.UserHomeDir(); th, _ := config.LoadTheme(home); wtapp.SetTheme(th)`.

- [ ] **Step 1: Write the failing tests.**
  - In `config_test.go`:
    - `TestLoadThemeDefaults`: with no file in a temp HOME, the result equals `DefaultTheme()`.
    - `TestLoadThemeOverrides`: a file containing `[theme]\noptions_text_color = magenta\n` gives `OptionsText == "magenta"` and leaves the other four fields at their defaults.
    - `TestLoadThemeBadValues`: `active_border_color = blu` is returned raw as `"blu"`. Parsing happens in wtapp.
  - In `theme_test.go`, use a helper `withColour(t)` that sets `lipgloss.SetColorProfile(termenv.ANSI256)`. It registers a `t.Cleanup` that restores the old profile and calls `SetTheme(config.DefaultTheme())`.
    - `TestParseSpec`, table-driven:
      - `"green, bold"` renders "x" equal to `lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true).Render("x")`
      - `"default"` renders plain "x"
      - `"#ff0000"` and `"196"` are both accepted
      - `"blu"`, `"300"`, `"#12"` and `"red, blue"` are errors
      - `"Blue , BOLD"` is accepted
    - `TestSetThemeWarns`: `SetTheme` with `ActiveBorder: "blu"` returns `theme: unknown colour "blu" in active_border_color, using default`, and `active` renders like the green bold default.
    - `TestFocusedBorderGreenBold`: with the fixture model at 120×40 and Worktrees focused, the Worktrees top border line contains `active.Render("╭─")`.
    - `TestUnfocusedBorderDefault`: the `[3] Context` top line does not contain `active.Render("╭─")`.
    - `TestFilterBorderCyan`: after `/` and then `x`, the Worktrees top line contains `searching.Render("╭─")`.
    - `TestHintLineBlue`: with no status message, the last line of `View()` contains `"\x1b[34m"`.
    - `TestPopupBorderGreen`: after `?`, the help popup's top line contains `active.Render("╭─")`.
    - `TestThemeFromConfig`: `SetTheme(config.Theme{…defaults, ActiveBorder: "magenta"})` makes the focused border contain `lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Render("╭─")`.
    - `TestThemeWarningInStatus`: after `SetTheme` with a bad value, `New(…)` has a `statusMsg` containing `theme: unknown colour`.
- [ ] **Step 2: Run** `go test ./internal/shared/config/ ./internal/gitwt/app/ -run 'Theme|ParseSpec|Border|HintLine' -v`. Expect FAIL: `LoadTheme`, `SetTheme` and `parseSpec` are undefined.
- [ ] **Step 3: Implement** the Interfaces above.
  - Wire the styles in: the Worktrees border uses `searching` while filtering, `active` goes to the popup and confirm-modal boxes, and the footer hint line uses `hintStyle`. The footer's status and busy branches keep their colours.
  - Call `SetTheme(config.DefaultTheme())` from a package `init` in wtapp, so tests and any caller that skips SetTheme still get the defaults.
- [ ] **Step 4: Run** `go test ./internal/gitwt/app/ ./internal/shared/config/ -v`, then `go vet ./... && go test ./...`. Expect PASS, with every pre-existing test passing unchanged.
- [ ] **Step 5: Commit** `internal/shared/config internal/gitwt/app internal/cli/wt_status.go go.mod go.sum` with message `feat(wtapp): lazygit colours, configurable via [theme] in ~/.devex.conf`.

### Task 2: Full-width selected-row highlight, plus docs

**Files:**
- Modify: `internal/shared/ui/box.go` (add `Highlight`).
- Test: `internal/shared/ui/box_test.go` (create it if it is missing).
- Modify:
  - `internal/gitwt/app/theme.go` (add the selected-row styles)
  - `internal/gitwt/app/layout.go` (the `listLines` selected row at about line 319, and the Context row at about line 151)
  - `internal/gitwt/app/popup.go` (the cursor row at about line 216)
  - `internal/gitwt/app/theme_test.go`
  - `README.md`: in the `git wt` app section, add a short "Colours" subsection. It says the defaults match lazygit, shows the `[theme]` block from Config format verbatim, and notes that bad values fall back to defaults with a status-line warning.

**Interfaces:**
- Consumes: Task 1's `SetTheme`, `parseSpec`, the `withColour(t)` test helper, and `config.Theme.SelectedLineBg`.
- Produces:
  - `func Highlight(s string, w int, st lipgloss.Style) string` in package ui:
    - It returns `s` fitted to exactly `w` cells with `Fit`, which truncates with `…` and pads with spaces.
    - `st` is applied to the whole row, padding included.
    - Every SGR reset inside `s` (`"\x1b[0m"` or `"\x1b[m"`) is followed again by `st`'s opening sequence, so the background reaches the end of the row and inner foreground colours survive.
    - The visible width is exactly `w`.
    - With an empty `st`, or under a no-colour profile, it returns exactly `Fit(s, w)`.
  - In package wtapp:
    - `selFocused`, set by `SetTheme` as `parseSpec(t.SelectedLineBg, true)` plus `.Bold(true)`. The default is a blue `"4"` background, bold.
    - `selBlurred = lipgloss.NewStyle().Bold(true)`.
  - Callers pass the panel's inner width: the `w` that `panelLines(p, w, h)` receives, which is the box width minus 2. The popup passes its inner box width.

- [ ] **Step 1: Write the failing tests.**
  - In `box_test.go`, with a local profile-forcing helper:
    - `TestHighlightWidth`: for each `s` in `["abc", strings.Repeat("界", 30), "a\x1b[33mmerged\x1b[0m b"]` and each `w` in `[1, 10, 40]`, `lipgloss.Width(Highlight(s, w, st)) == w`.
    - `TestHighlightKeepsBackgroundAfterReset`: let `s := "x " + lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Render("merged") + " y"`, `st := lipgloss.NewStyle().Background(lipgloss.Color("4"))` and `out := Highlight(s, 20, st)`.
      - Every reset in `out` except the last is immediately followed by `st`'s opening SGR. The opening SGR is the prefix of `st.Render("x")` before `"x"`.
      - `out` still contains the yellow `33` SGR.
    - `TestHighlightNoColour`: under `termenv.Ascii`, `Highlight(s, 10, st) == Fit(s, 10)`.
  - In `theme_test.go`:
    - `TestSelectedRowHighlightFollowsFocus`: at 120×40 with Worktrees focused, the selected worktree line contains the `44` background SGR. After pressing `3`, that line no longer contains `44`, and the selected todo line in Context does. Add one todo in the fixture.
    - `TestPopupCursorHighlighted`: build a filterable choose popup with 3 options the same way `popup_test.go` does. Exactly one line of its view, the cursor row, contains `44`.
    - `TestSelectedBgFromConfig`: `SetTheme` with `SelectedLineBg: "magenta"` makes the selected worktree line contain the `45` SGR.
- [ ] **Step 2: Run** `go test ./internal/shared/ui/ ./internal/gitwt/app/ -run 'Highlight|FollowsFocus|PopupCursor|SelectedBg' -v`. Expect FAIL: `Highlight`, `selFocused` and `selBlurred` are undefined.
- [ ] **Step 3: Implement `Highlight` and the styles, then wire them in.**
  - **Worktrees selected row:** build the row as today, including `▸`. Wrap it with `ui.Highlight(row, w, selFocused)` when `m.focus == panelWorktrees && !m.focusMain`, and with `ui.Highlight(row, w, selBlurred)` otherwise.
  - **Context row under the cursor:** use the same pattern, gated on `m.focus == panelContext`. Keep today's rule that `▸` shows only when Context is focused.
  - **Popup cursor row:** `ui.Highlight(mark+row, innerW, selFocused)`.
  - Leave all other rows unchanged.
  - Write the README Colours subsection.
- [ ] **Step 4: Run** `go test -race ./internal/gitwt/app/ ./internal/shared/ui/ ./internal/shared/config/ -v`, then `go vet ./... && go test ./...`. Expect PASS. The existing `TestWidthEverySize` sweep must still pass.
  - Manual check: run `go build -o dist/dx ./cmd/dx && ln -sf dx dist/git-wt` and open the app in a scratch bare repo, not this repo. Use `HOME=<scratch>` with a `[theme]` that sets `active_border_color = magenta`, and run tmux `capture-pane -e`. Confirm the focused border is magenta and the blue row spans the panel. Then use a bad value and confirm the warning shows in the status line.
- [ ] **Step 5: Commit** `internal/shared/ui internal/gitwt/app README.md` with message `feat(wtapp): lazygit-style selected-row highlight`.
