# git wt app: lazygit colour scheme Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the `git wt` full-screen app use lazygit's default colour scheme. The focused border is green and bold, other borders use the terminal's default colour, key hints are blue, the filter border is cyan, and popups have green borders. The selected row in Worktrees, Context and popup lists gets a full-width highlight.

**Architecture:**
- **Colours:** every colour lives in the one style block in `internal/gitwt/app/app.go` (the `var ( fg … )` block and `edge()`), so part 1 is a change to that block plus the few call sites that pick a border.
- **Row highlight:** part 2 adds one helper, `ui.Highlight`, in `internal/shared/ui`. It pads a row to the panel's inner width and paints it, keeping the colours already inside the row. The Worktrees list, the Context list and the popup list call it on their selected row.

**Tech Stack:** Go 1.27, lipgloss v1.1.0, `github.com/charmbracelet/x/ansi` (already direct), `github.com/muesli/termenv` v0.16.0. termenv is already in go.mod as indirect. Tests import it to force a colour profile, so `go mod tidy` makes it direct. No new modules.

**Spec:** This plan is its own spec. The owner asked for "the same lazygit colour scheme" for `git wt` only, and chose part 1 (colours) plus part 2 (selected-row highlight) from the estimate given on 2026-10-31. The values come from `lazygit --config` (lazygit 0.66.2) `gui.theme`:
- `activeBorderColor: [green, bold]`
- `inactiveBorderColor: [default]`
- `searchingActiveBorderColor: [cyan, bold]`
- `optionsTextColor: [blue]`
- `selectedLineFgColor: [bold]`

lazygit's built-in default for the focused view's selected-line background is blue. An unfocused view's selected line is only bold.

## Global Constraints

- Only `git wt`, meaning `internal/gitwt/app`, plus the new helper in `internal/shared/ui`. The `dx skills` grid (`internal/skills/app`) does not change.
- Exact colours, as ANSI 16-colour indexes so they follow the user's terminal palette the way lazygit does:
  - focused border: green `"2"`, bold
  - unfocused border: no colour (the terminal default)
  - filtering Worktrees border: cyan `"6"`, bold
  - key-hint line: blue `"4"`
  - popup and confirm-modal border: green `"2"`, bold
  - selected row in the focused panel or popup: background blue `"4"` and bold
  - selected row in an unfocused panel: bold only
- The `◆` focus marker and the `▸` row marker stay, so focus and selection still show on terminals without colour (NO_COLOR, dumb terminals). Existing tests rely on them.
- No rendered line may exceed the terminal width at any size of 60×12 or more. The minimum-size message `Terminal too small (need 60×12)` stays.
- Status-line colours (red errors, yellow `running…`) and text colours inside panels (yellow `merged`, green `[✓]`, dim notes) do not change.
- Never run host `git` in this repo. Commit only via `docker run --rm -v "$PWD":/devex-manager -w /devex-manager personal-git:v3 bash -c '…'`. No Co-Authored-By line. Never push.
- Tests: `go vet ./... && go test ./...` and `go test -race ./internal/gitwt/app/ ./internal/shared/ui/`.

## Review Focus

1. **A coloured segment inside the selected row** (`merged` in yellow, `[✓]` in green, `↑2` in yellow): the blue background must continue across the whole row after each segment. An inner style's SGR reset must not cut the highlight short. Test in Task 2 (`TestHighlightKeepsBackgroundAfterReset`).
2. **A selected row longer than the panel, or with wide characters (CJK, emoji):** the highlighted row must be exactly the inner width, truncated with `…`, never wider. Test in Task 2 (`TestHighlightWidth`), and through the existing `TestWidthEverySize` sweep in Task 2.
3. **A terminal without colour, and tests without a TTY:** lipgloss renders plain text there. The app must still show focus (`◆`) and selection (`▸`), and every existing test must pass unchanged. Covered by the existing suite. New colour assertions force a colour profile in the test and restore it afterwards.
4. **Focus moving while a row is selected:** only the focused panel's selected row gets the blue background. The other panels' selected rows are bold only. Test in Task 2 (`TestSelectedRowHighlightFollowsFocus`).
5. **A popup opened over the panels:** the popup border is green and its cursor row is highlighted. The panel underneath keeps its own focus colours, and nothing overflows at 60×12. Test in Task 2 (`TestPopupCursorHighlighted`) and the existing popup width sweep.

---

## File Structure

```
internal/gitwt/app/app.go        style block + edge(): lazygit colours (Task 1)
internal/gitwt/app/layout.go     filter border, hint line colour (Task 1); selected-row highlight in Worktrees/Context (Task 2)
internal/gitwt/app/popup.go      popup border (Task 1); cursor-row highlight (Task 2)
internal/gitwt/app/theme_test.go colour assertions with a forced colour profile (Tasks 1–2)
internal/shared/ui/box.go        + Highlight (Task 2)
internal/shared/ui/box_test.go   Highlight tests (Task 2)
README.md                        one line: the app uses lazygit's default colours (Task 2)
```

### Task 1: lazygit border and hint colours

**Files:**
- Modify: `internal/gitwt/app/app.go:549-566`. That is the style block and `edge()`. Also the confirm modal's `ui.Box(… accent)` at about line 637.
- Modify: `internal/gitwt/app/layout.go`. The worktree box border while `m.filtering` is set or `m.filter != ""`, and `footer()`'s hint-line style at about line 278.
- Modify: `internal/gitwt/app/popup.go:236`, the popup `ui.Box(…, accent)`.
- Test: `internal/gitwt/app/theme_test.go`, new.

**Interfaces:**
- Produces, in package wtapp:
  - `active = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)`
  - `inactive = lipgloss.NewStyle()`
  - `searching = …Color("6")…Bold(true)`
  - `hintStyle = fg("4")`
- `edge(on bool)` returns `active` or `inactive`.
- `accent` stays, because the `▸` marker uses it. Its colour becomes green `"2"`, so the marker matches the focused border.
- Task 2 uses `active` for nothing new. It defines its own selected-row styles.

- [ ] **Step 1: Write the failing tests** in `theme_test.go`. Use a helper `withColour(t)` that calls `lipgloss.SetColorProfile(termenv.ANSI)` and restores the previous profile with `t.Cleanup`. Then, using the fixture model and `WindowSizeMsg{120, 40}` as in `layout_test.go`:
  - `TestFocusedBorderGreenBold`: focus `[2] Worktrees`. The Worktrees box's top border line contains `active.Render("╭─")` as a substring, so the test doesn't depend on SGR ordering.
  - `TestUnfocusedBorderDefault`: with Worktrees focused, the `[3] Context` box's top line contains `"╭─"` with no SGR immediately before it that sets a colour. Concretely, `active.Render("╭─")` is not in that line and the plain `"╭─"` is.
  - `TestFilterBorderCyan`: press `/` and type `x`. The Worktrees top border contains `searching.Render("╭─")`.
  - `TestHintLineBlue`: the last line of `View()`, with no status message set, contains the blue foreground SGR `"\x1b[34m"`.
  - `TestPopupBorderGreen`: press `?`. The help popup's top border contains `active.Render("╭─")`.
- [ ] **Step 2: Run** `go test ./internal/gitwt/app/ -run 'Border|HintLine' -v`. Expect FAIL, either undefined `active`/`searching`/`hintStyle` or a missing substring.
- [ ] **Step 3: Implement.**
  - Change the style block and `edge()` as in Interfaces.
  - Pass `searching` as the Worktrees border while filtering.
  - Pass `active` to the popup `ui.Box` and the confirm-modal `ui.Box`.
  - Render the footer hint line with `hintStyle` instead of `dim`. The status-message and busy branches of `footer()` keep their own colours.
- [ ] **Step 4: Run** `go test ./internal/gitwt/app/ -v`, then `go vet ./... && go test ./...`. Expect PASS. Every pre-existing test passes unchanged.
- [ ] **Step 5: Commit** `internal/gitwt/app go.mod go.sum` with message `feat(wtapp): lazygit border and hint colours`.

### Task 2: Full-width selected-row highlight

**Files:**
- Modify: `internal/shared/ui/box.go`, adding `Highlight`.
- Test: `internal/shared/ui/box_test.go`. Create it if it doesn't exist.
- Modify: `internal/gitwt/app/layout.go`. The `listLines` selected row at about line 319, and the Context row at about line 151.
- Modify: `internal/gitwt/app/popup.go`, the cursor row at about line 216.
- Modify: `internal/gitwt/app/theme_test.go`.
- Modify: `README.md`. Add one sentence to the `git wt` app section saying it uses lazygit's default colours: green focused border, blue selected row.

**Interfaces:**
- Consumes: Task 1's `withColour(t)` test helper.
- Produces:
  - `func Highlight(s string, w int, st lipgloss.Style) string` in package ui.
    - It returns `s` fitted to exactly `w` cells, using `Fit`, so it truncates with `…` and pads with spaces.
    - `st` applies to the whole row, padding included.
    - Every SGR reset inside `s` (`"\x1b[0m"` and `"\x1b[m"`) is followed by `st`'s opening sequence again, so the background continues to the end and inner foreground colours stay.
    - The visible width is exactly `w`.
    - With an empty `st`, or under a no-colour profile, it returns exactly `Fit(s, w)`.
  - In package wtapp:
    - `selFocused = lipgloss.NewStyle().Background(lipgloss.Color("4")).Bold(true)`
    - `selBlurred = lipgloss.NewStyle().Bold(true)`
  - The callers must pass the panel's inner width. This is the `w` that `panelLines(p, w, h)` receives, which is `box width - 2`. The popup passes its inner box width.

- [ ] **Step 1: Write the failing tests.**
  - In `box_test.go`, with `withColour`-style profile forcing (a local helper, since this is package ui):
    - `TestHighlightWidth`: for `s` in `["abc", strings.Repeat("界", 30), "a\x1b[33mmerged\x1b[0m b"]` and `w` in `[1, 10, 40]`, check that `lipgloss.Width(Highlight(s, w, st)) == w`.
    - `TestHighlightKeepsBackgroundAfterReset`: take `s := "x " + lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Render("merged") + " y"`, `st := lipgloss.NewStyle().Background(lipgloss.Color("4"))` and `out := Highlight(s, 20, st)`.
      - Every reset in `out` except the last is immediately followed by `st`'s opening SGR. Compute that opening SGR as the prefix of `st.Render("x")` up to `"x"`.
      - `out` contains the yellow SGR, `33`, so the inner colour is kept.
    - `TestHighlightNoColour`: under `termenv.Ascii`, `Highlight(s, 10, st) == Fit(s, 10)`.
  - In `theme_test.go`:
    - `TestSelectedRowHighlightFollowsFocus`: at 120×40 with Worktrees focused, the selected worktree's line contains the opening SGR of `selFocused`. Press `3` to focus Context. Now the Worktrees selected line does not contain the blue-background SGR (`44`), but does contain bold. If the fixture has a todo, the Context selected todo line contains the `44` SGR.
    - `TestPopupCursorHighlighted`: open a filterable choose popup with 3 options. Drive `n` with a stubbed branches message, or build the popup directly as `popup_test.go` does. The cursor row line contains the `44` SGR and no other row does.
- [ ] **Step 2: Run** `go test ./internal/shared/ui/ ./internal/gitwt/app/ -run 'Highlight|HighlightFollowsFocus|PopupCursorHighlighted' -v`. Expect FAIL with undefined `Highlight`, `selFocused` and `selBlurred`.
- [ ] **Step 3: Implement `Highlight`**, then wire it in:
  - **Worktrees selected row:** build the row exactly as today, including the `▸` mark, and wrap it with `ui.Highlight(row, w, selFocused)` when `m.focus == panelWorktrees && !m.focusMain`. Otherwise wrap it with `ui.Highlight(row, w, selBlurred)`. The existing `bold.Render(name)` on the name then becomes redundant. Keep it or drop it, but keep the output width the same.
  - **Context todo under the cursor:** the same pattern, gated on `m.focus == panelContext`. Today `▸` only shows when Context is focused. Keep that, and give the blurred cursor row `selBlurred`.
  - **Popup cursor row:** `ui.Highlight(mark+row, innerW, selFocused)`. A popup always has focus.
  - Leave every non-selected row unchanged.
- [ ] **Step 4: Run** `go test -race ./internal/gitwt/app/ ./internal/shared/ui/ -v`, then `go vet ./... && go test ./...`. Expect PASS. The existing `TestWidthEverySize` sweep (60–130 × 12–40, all focus and popup states) still passes, which proves width safety with the highlight in place. Then do a manual check: `go build -o dist/dx ./cmd/dx && ln -sf dx dist/git-wt`, open it in a scratch bare repo (not this repo), and use tmux `capture-pane -e` to confirm the blue row spans the full panel width.
- [ ] **Step 5: Commit** `internal/shared/ui internal/gitwt/app README.md` with message `feat(wtapp): lazygit-style selected-row highlight`.
