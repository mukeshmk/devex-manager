# git wt lazygit-style app Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Run `s` sync, `c` clean and `d` delete inside the `git wt` full-screen app through popups, with a lazygit-style panel layout (Status, Worktrees, Context, Sync, main view, command log) and in-app `dx ctx` todo editing.

**Architecture:** The bodies of the command-line `runSync`, `runClean` and `runRm` move into a new `internal/gitwt/flows` package that takes an `Env` (cwd, output writer, error writer, prompter, TTY flag). The CLI builds an `Env` from the process streams. The app builds one whose prompter turns each question into a popup and whose writer feeds the command log. Each flow runs in a goroutine and talks to the Bubble Tea model through one event channel. `gitx.Run` gets a goroutine-safe hook so every git command lands in the command log.

**Tech Stack:** Go 1.27, bubbletea v1.3.6, lipgloss v1.1.0, bubbles (textinput/viewport, same family), cobra. No new dependencies outside the Charm family.

**Spec:** `docs/superpowers/specs/2026-10-30-lazygit-style-app-design.md`

## Global Constraints

- Command-line output, flags, exit codes, safety rules, backups and config do not change. Every existing `internal/cli` test passes untouched (only import changes allowed).
- Every in-app action calls the same `flows` function as its command-line counterpart. No safety check is duplicated or skipped in the app.
- Never run host `git` against this repo. Commit via the container:
  ```bash
  cd /Users/MARAMBAK/personal/code/devex-manager && docker run --rm -v "$PWD":/devex-manager -w /devex-manager personal-git:v3 bash -c 'git add <files> && git commit -q -m "<msg>" && git log --oneline -1'
  ```
- Commit messages contain no `Co-Authored-By` or Claude attribution. Never push.
- Folder names differ from package names: `internal/gitwt/worktree` is package `wt`, `internal/gitwt/sync` is package `wtsync`, `internal/gitwt/app` is package `wtapp`. The new `internal/gitwt/flows` is package `flows`.
- Charm majors stay bubbletea v1, lipgloss v1, bubbles v0/v1 (whatever `go get` resolves for the v1 family).
- Minimum terminal size stays `Terminal too small (need 60×12)`. No rendered line may exceed the terminal width.
- Test command: `go test ./...` from the repo root, plus `go test -race ./internal/gitwt/... ./internal/shared/gitx/` where named.

## Review Focus

1. **Quit during a running flow:** `q` and ctrl+c (outside a popup) while sync, clean or rm runs must not quit. Expect a `wait for <flow> to finish` status and no half-applied state. Test in Task 6.
2. **ctrl+c or esc inside a sync choice popup:** it must abort the whole sync before Apply (`ErrAborted`), and nothing changes on disk. Test in Task 6.
3. **Concurrent git calls feeding the command log:** `Collect` runs a goroutine per worktree, so the hook must be race-free. Test with `-race` in Task 1.
4. **Popup or panels at small or changing sizes:** resizing to 60×12 with a popup open still renders within the width, and the popup truncates. Test in Task 4 and Task 5.
5. **Command log growth:** the log is a bounded ring of 500 lines. Test in Task 3.

---

## File Structure

```
internal/shared/gitx/git.go          + SetCommandHook(func(Command)) / Command type
internal/gitwt/flows/env.go          Env, ErrReported
internal/gitwt/flows/sync.go         Sync(e Env, o SyncOpts) error        (body of cli.runSync)
internal/gitwt/flows/clean.go        Clean(e Env, yes bool) error         (body of cli.runClean)
internal/gitwt/flows/rm.go           Remove(e Env, target string, force bool) error (body of cli.runRm + refuseUnsafe)
internal/gitwt/flows/flows_test.go
internal/cli/wt_sync.go, wt_clean.go, wt_rm.go   thin wrappers building Env
internal/gitwt/app/bridge.go         event channel, chanPrompter, logWriter, runFlow
internal/gitwt/app/popup.go          popup model: confirm / choose / checklist / input
internal/gitwt/app/layout.go         panel geometry + rendering (Status, Worktrees, Context, Sync, main, log)
internal/gitwt/app/app.go            model, keys, actions (paused + self-exec removed)
internal/gitwt/app/*_test.go
README.md, CLAUDE.md                 keys, layout, lazygit customCommands snippet
```

---

### Task 1: Command hook in gitx

**Files:**
- Modify: `internal/shared/gitx/git.go` (`Run`, and `ChangedFiles` which builds its own `exec.Command`)
- Test: `internal/shared/gitx/hook_test.go`

**Interfaces:**
- Produces:
  - `type Command struct { Dir string; Args []string; Err error; Duration time.Duration }`
  - `func SetCommandHook(fn func(Command))`. `nil` disables the hook. The setter and the call are guarded by a `sync.RWMutex`. The hook runs after each git process exits, from whichever goroutine ran it.

- [ ] **Step 1: Write failing tests**
  - `TestCommandHookSeesRuns`: set the hook, run `Run(tmp, "--version")`, and check the hook got `Args == ["--version"]`, `Err == nil`, and `Dir == tmp`.
  - `TestCommandHookConcurrent`: 20 goroutines call `Run(tmp, "--version")` while the hook appends to a mutex-guarded slice. Expect 20 entries.
  - Both reset the hook with `t.Cleanup(func(){ SetCommandHook(nil) })`.
- [ ] **Step 2: Run** `go test -race ./internal/shared/gitx/ -run CommandHook -v`. Expect FAIL (undefined).
- [ ] **Step 3: Implement.** One unexported `notify(cmd Command)` is called from both `Run` and `ChangedFiles`.
- [ ] **Step 4: Run** the same command. Expect PASS with no race reports, then run `go test ./...`.
- [ ] **Step 5: Commit** `internal/shared/gitx` with message `feat(gitx): command hook for the app's command log`.

### Task 2: Extract the sync, clean and rm flows

**Files:**
- Create: `internal/gitwt/flows/env.go`, `sync.go`, `clean.go`, `rm.go`, `flows_test.go`
- Modify: `internal/cli/wt_sync.go`, `wt_clean.go`, `wt_rm.go`. They become wrappers, and `refuseUnsafe` moves to `flows/rm.go`.

**Interfaces:**
- Produces:
  - `type Env struct { Cwd string; Out, Err io.Writer; Prompter ui.Prompter; TTY bool }`. `TTY` means "may ask questions". The CLI sets it from `ui.IsTTY(stdin)`, and the app always sets it true.
  - `var ErrReported = errors.New("reported")`: the flow already printed its message, so the caller exits 1 without printing again. The CLI maps it to `errExit1`.
  - `type SyncOpts struct { Check, DryRun, Yes bool }`
  - `func Sync(e Env, o SyncOpts) error`: the body of today's `runSync`, with `os.Getwd()` replaced by `e.Cwd`. On a TTY it uses `e.Prompter`, which replaces the inline `ui.NewPrompter(in, out)`. Otherwise it uses `ui.DefaultPrompter{}`. It still returns `wtsync.ErrAborted` wrapped so callers can tell an abort apart (`errors.Is`).
  - `func Clean(e Env, yes bool) error`: the body of `runClean`. With `--yes` it uses `DefaultPrompter`, otherwise `e.Prompter`. The CLI passes `ui.NewPrompter(in, out)` as `e.Prompter`, which is a LinePrompter off a TTY, so `echo y |` keeps working.
  - `func Remove(e Env, target string, force bool) error`: the body of `runRm` for a single target, plus `refuseUnsafe`. Usage and arg-count errors stay in the CLI wrapper.
- CLI wrappers: `runSync(in, out, errw, check, dryRun, yes)` builds `Env{Cwd: os.Getwd(), Out: out, Err: errw, Prompter: ui.NewPrompter(in, out), TTY: ui.IsTTY(in)}` and maps `ErrReported`/aborts to `errExit1`. Same for clean and rm.

- [ ] **Step 1: Write failing tests** in `flows_test.go` against `testutil.NewRepo`, each with a scripted prompter that records the questions it was asked:
  - `TestSyncFlowUsesEnvPrompter`: on a conflict, the scripted prompter is asked a Choose whose options include `Resolve Later`.
  - `TestSyncFlowAbort`: the prompter's `Aborted()` returns true, so `errors.Is(err, wtsync.ErrAborted)` and the files are unchanged.
  - `TestCleanFlowPreselectsSafe`: the MultiSelect `pre` slice is true only for safe candidates.
  - `TestRemoveFlowRefusesDirty`: returns `ErrReported`, `e.Err` contains `Uncommitted changes`, and the worktree still exists.
- [ ] **Step 2: Run** `go test ./internal/gitwt/flows/ -v`. Expect FAIL (package missing).
- [ ] **Step 3: Implement.** Move the bodies (don't copy them) and turn the CLI functions into wrappers. Keep every output string identical.
- [ ] **Step 4: Run** `go test ./internal/gitwt/flows/ ./internal/cli/ -v`. Expect PASS, with every pre-existing cli test unchanged and passing. Then run `go test ./...`.
- [ ] **Step 5: Commit** `internal/gitwt/flows internal/cli` with message `refactor(wt): shared sync/clean/rm flows for CLI and app`.

### Task 3: App event bridge

**Files:**
- Create: `internal/gitwt/app/bridge.go`, `bridge_test.go`

**Interfaces:**
- Consumes: `ui.Prompter` and the Task 2 `flows.Env`.
- Produces:
  - `type event interface{}` delivered on `events chan tea.Msg`, held by the model. `func waitEvent(ch chan tea.Msg) tea.Cmd` returns the next message. The model re-issues it after every event (the standard Bubble Tea channel pattern).
  - Messages:
    - `promptMsg{Kind promptKind /* confirm|choose|checklist */; Question string; Options []string; Def int; Pre []bool; reply chan promptAnswer}` and `promptAnswer{Bool bool; Index int; Ticks []bool; Abort bool}`
    - `logMsg{Line string}`
    - `flowDoneMsg{Name string; Err error}`
  - `type chanPrompter struct{ ch chan tea.Msg; aborted bool }` implements `ui.Prompter`. Each call sends a `promptMsg` and blocks on `reply`. If an answer has `Abort` set, it returns the safe default, marks `aborted`, and every later call returns its default immediately without asking.
  - `type logWriter struct{ ch chan tea.Msg; buf []byte }` implements `io.Writer`. It splits on `\n`, strips ANSI with `ansi.Strip`, and sends one `logMsg` per line.
  - `func runFlow(ch chan tea.Msg, name string, fn func(flows.Env) error, cwd string) tea.Cmd` starts a goroutine with `Env{Cwd: cwd, Out: lw, Err: lw, Prompter: cp, TTY: true}`, then sends `flowDoneMsg`.
  - `type logRing struct{ lines []string }` with `Add(line string)` keeps the last 500 lines, and `Lines() []string`.

- [ ] **Step 1: Write failing tests:**
  - `TestChanPrompterRoundTrip`: a goroutine calls `Choose("q", ["a","b"], 1)`. The test reads `promptMsg` from the channel, replies `Index: 0`, and the call returns 0.
  - `TestChanPrompterAbortSticks`: the first reply has `Abort` set, so the call returns the default and `Aborted()` is true. A second `Confirm` returns `def` without sending.
  - `TestLogWriterSplitsAndStrips`: writing `"\x1b[31mred\x1b[0m\nhalf"` then `" line\n"` gives the messages `red` and `half line`.
  - `TestLogRingBounded`: after 600 adds, `len(Lines()) == 500` and the first line is line #101.
- [ ] **Step 2: Run** `go test -race ./internal/gitwt/app/ -run 'ChanPrompter|LogWriter|LogRing' -v`. Expect FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** the same command. Expect PASS with no race reports.
- [ ] **Step 5: Commit** `internal/gitwt/app/bridge*.go` with message `feat(wtapp): event bridge for in-app flows`.

### Task 4: Popup component

**Files:**
- Create: `internal/gitwt/app/popup.go`, `popup_test.go`

**Interfaces:**
- Produces:
  - `type popup struct { kind popupKind /* confirm|choose|checklist|input|help */; title string; options []string; cursor int; ticks []bool; input textinput.Model; reply chan promptAnswer; onSubmit func(string) tea.Cmd }`
  - `func (p popup) update(msg tea.KeyMsg) (popup, done bool, answer promptAnswer, cmd tea.Cmd)`. Keys by kind:
    - confirm: `enter`/`y` give `Bool:true`, `esc`/`n` give `Bool:false`.
    - choose: `↑/↓/j/k`, and `enter` gives `Index`. `esc` gives `Index: def`.
    - checklist: `space` ticks, `a` toggles all, `enter` gives `Ticks`, `esc` gives all-false.
    - input: `enter` calls `onSubmit(value)`, `esc` closes.
    - help: any key closes.
    - `ctrl+c` gives `Abort:true` for every kind.
  - `func (p popup) view(maxW, maxH int) string`: a bordered box centred by the caller. Its width is `min(maxW-4, max(40, longest line + 4))`, lines are truncated with `ui.Fit`, and a checklist or choose list taller than `maxH-4` scrolls with the cursor.

- [ ] **Step 1: Write failing tests:**
  - `TestPopupConfirmKeys` (enter, y, n, esc)
  - `TestPopupChooseDefaultOnEsc`
  - `TestPopupChecklistTicks` (space twice gives false, `a` gives all true)
  - `TestPopupCtrlCAborts`, for every kind
  - `TestPopupViewFits`: 20 options of 120 chars at maxW 60, maxH 12. Every line is at most 56 cells and there are at most 12 lines.
- [ ] **Step 2: Run** `go test ./internal/gitwt/app/ -run Popup -v`. Expect FAIL.
- [ ] **Step 3: Implement.** Add `github.com/charmbracelet/bubbles/textinput` if it isn't present yet.
- [ ] **Step 4: Run** the same command. Expect PASS.
- [ ] **Step 5: Commit** `internal/gitwt/app/popup*.go go.mod go.sum` with message `feat(wtapp): lazygit-style popups`.

### Task 5: Lazygit-style layout

**Files:**
- Create: `internal/gitwt/app/layout.go`, `layout_test.go`
- Modify: `internal/gitwt/app/app.go`. Replace `View`, `listLines`, `detailLines`, `header` and `footer` with the panel rendering. Keep the data loading as it is.

**Interfaces:**
- Produces:
  - `type panel int` with `panelStatus=1, panelWorktrees=2, panelContext=3, panelSync=4, panelLog=5`, plus `focusMain bool`.
  - `func layout(w, h int) (left []rect, main, log rect, stacked bool)`. Wide (w ≥ 80): the left column takes `w/3`, Status 4 rows, and the remaining rows split Worktrees 50%, Context 25%, Sync 25%. The main view takes 70% of the right column's height and the log the rest. Narrow (60 ≤ w < 80): stacked, with the log at 3 rows. One row is reserved for the key-hint line.
  - The model also gains:
    - `syncLines []string`: from `SyncSession.InSync()`, loaded with the rows; empty if there are no copy paths.
    - `statusMsg string` and `statusErr bool`.
    - `log logRing`, `popup *popup`, and `busy string` (the name of the running flow).
  - Panel contents:
    - **Status:** `<repo> · base <branch>` and `sync: in sync | N files out of sync | no copy paths`.
    - **Worktrees:** today's list rows.
    - **Context:** todos of the selected worktree's branch (`[ ]`/`[✓]`), or `No todos`.
    - **Sync:** the `Out of sync:` lines.
  - Main view by focused panel, as in the spec. It scrolls with `↑/↓` when `focusMain`.
  - Key-hint line: hints for the focused panel, ending with `? keys`, cut to the width.
  - `?` opens the help popup listing every key.
  - Focused panel border uses the accent style and the others are dim. Titles look like `[2] Worktrees`.

- [ ] **Step 1: Write failing tests**, driving the model with `WindowSizeMsg` and a pre-loaded rows message:
  - `TestLayoutWide` at 120×40: the output has `[1] Status`, `[2] Worktrees`, `[3] Context`, `[4] Sync` and `Command log`, and every line is at most 120.
  - `TestLayoutStacked` at 70×30: every line is at most 70, and all five titles are present.
  - `TestLayoutTooSmall` at 50×10: `Terminal too small (need 60×12)`.
  - `TestPanelKeys`: `3` focuses Context (its border uses the focus style, so assert the title is rendered with the focus marker). `tab` moves to 4, `shift+tab` back to 3, `enter` sets focusMain, `esc` clears it.
  - `TestResizeWithPopupOpen`: open help (`?`), resize to 60×12, and every line is at most 60.
- [ ] **Step 2: Run** `go test ./internal/gitwt/app/ -run 'Layout|PanelKeys|Resize' -v`. Expect FAIL.
- [ ] **Step 3: Implement.** Reuse `ui.Box` and `ui.Fit` for every panel.
- [ ] **Step 4: Run** `go test ./internal/gitwt/app/ -v`. Existing tests whose assertions describe the old layout get updated to the new titles. Their behaviour assertions (todo toggle, filter, delete confirm contents) must keep passing in spirit.
- [ ] **Step 5: Commit** `internal/gitwt/app` with message `feat(wtapp): lazygit-style panel layout`.

### Task 6: In-app sync, clean and delete

**Files:**
- Modify: `internal/gitwt/app/app.go`. Delete `paused`, `run`, the `self` field and the injectable exec seam.
- Modify: `internal/cli/wt_status.go` (drop the `self`/`os.Executable` plumbing, and pass cwd in instead).
- Test: `internal/gitwt/app/actions_test.go`

**Interfaces:**
- Consumes: Task 2 `flows.Sync/Clean/Remove`, Task 3 bridge, Task 4 popup, Task 5 model fields.
- `New(repo *gitx.Repo, cfg config.RepoConfig, cwd string) tea.Model`. `Run(m)` also installs `gitx.SetCommandHook`, which forwards `logMsg{"git " + args}` into the model's channel, and removes it on exit.
- Keys:
  - `s` runs `runFlow(ch, "sync", func(e){ return flows.Sync(e, flows.SyncOpts{}) }, cwd)`. Before it starts, if the plan has steps, the flow's own `Confirm("Apply these changes?")` arrives as a popup. That works because `Env.TTY` is true.
  - `c` runs `flows.Clean(e, false)`. Its MultiSelect arrives as a checklist popup.
  - `d` on the selected worktree keeps today's pre-check modal. It shows what would be lost and refuses unsafe, main or base worktrees with close only. On confirm it runs `flows.Remove(e, row.WT.Path, false)`.
- While `busy != ""`:
  - Only popup keys are handled.
  - `q`/ctrl+c outside a popup sets `statusMsg = "wait for <busy> to finish"` and does not quit.
  - `s/c/d` are ignored.
- `promptMsg` opens the matching popup with its `reply` channel. The popup answer is sent on `reply`.
- `flowDoneMsg` clears `busy`, sets `statusMsg` (`sync: done`, or `sync aborted, nothing changed`, or the error with `statusErr`), and reloads rows and sync lines.
- `logMsg` goes into `log`.

- [ ] **Step 1: Write failing tests.** Use a fixture repo with copy paths and a model driven through `Update`. Pump the channel in tests with a helper `drain(m, n)` that reads up to n messages from `m.events`, with a timeout.
  - `TestAppSyncInApp`: modify a copy-path file in a feature worktree and press `s`.
    - A confirm popup appears, and `enter` answers it.
    - After `flowDoneMsg`, the file is in main and `statusMsg` contains `sync: done`.
    - The command log contains a line with `Synchronization complete`.
  - `TestAppSyncAbortInPopup`: set up a conflict, press `s`, then `enter` on confirm, then `ctrl+c` on the conflict choose popup. Expect `sync aborted`, and files unchanged.
  - `TestAppQuitBlockedWhileBusy`: while a prompt is pending, `q` returns no `tea.Quit` and `statusMsg` contains `wait for sync to finish`.
  - `TestAppCleanChecklist`: a merged clean worktree plus a dirty merged one. `c` opens a checklist with only the clean one ticked, and `enter` removes only it.
  - `TestAppDeleteInApp`: `d` then `y` on a clean worktree removes its folder and branch. On a dirty one, `y` does nothing (as today).
  - `TestAppNoCopyPaths`: with no copy paths, `s` logs `No copy paths configured…` and `statusMsg` shows it. No popup and no shell.
- [ ] **Step 2: Run** `go test -race ./internal/gitwt/app/ -run 'AppSync|AppQuit|AppClean|AppDelete|AppNoCopy' -v`. Expect FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test -race ./internal/gitwt/app/ -v`, then `go test ./...`. Manual check: build to `dist/`, run `git wt` in a scratch bare repo (not this repo), and press `s`, `c` and `d`. Nothing leaves the alt screen.
- [ ] **Step 5: Commit** `internal/gitwt/app internal/cli` with message `feat(wtapp): sync, clean and delete run in-app with popups`.

### Task 7: Context panel actions

**Files:**
- Modify: `internal/gitwt/app/app.go`
- Test: `internal/gitwt/app/context_test.go`

**Interfaces:**
- Consumes: `ctx.Path`, `ctx.EnsureFile`, `ctx.AddTodo`, `ctx.SetDone`, `ctx.RemoveTodo`, `ctx.ClearDone`, `ctx.Write`. Each edit writes atomically, like `dx ctx`.
- Keys, only when the Context panel is focused:
  - `a` opens an input popup titled `New todo for <branch>`. Enter adds the todo, and the context file is created with `EnsureFile` if it's missing.
  - `space` toggles the todo under the Context cursor.
  - `x` opens a confirm `Delete todo "<text>"?`, and yes removes it.
  - `C` opens a confirm `Clear N done todo(s)?`, and yes clears them.
  - A detached worktree shows `No todos (detached HEAD)` and the keys do nothing.
  - Each edit logs `ctx: <action> <text>` in the command log.

- [ ] **Step 1: Write failing tests:**
  - `TestCtxAddTodo`: `3`, `a`, type `ship it`, `enter`. The file now contains `- [ ] ship it`.
  - `TestCtxToggleAndDelete`
  - `TestCtxClearDone`
  - `TestCtxDetachedNoop`
- [ ] **Step 2: Run** `go test ./internal/gitwt/app/ -run Ctx -v`. Expect FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/gitwt/app/ -v` and `go test ./...`. Expect PASS.
- [ ] **Step 5: Commit** `internal/gitwt/app` with message `feat(wtapp): edit dx ctx todos in the Context panel`.

### Task 8: Docs

**Files:**
- Modify: `README.md`, covering:
  - the `git wt` app section, with the layout sketch and a key table grouped by panel
  - a note that sync, clean and delete run in-app
  - a "Using with lazygit" section with the `customCommands` snippet: `git wt` with `subprocess: true`, and `dx skills` with `subprocess: true`
- Modify: `CLAUDE.md`, with the `flows` package in the layout table, and a line saying in-app actions must call `flows` and never duplicate safety logic.

- [ ] **Step 1:** Update both files. Every key in the README must match the `helpItems`/help popup in code. Check with `grep` for each documented key in `internal/gitwt/app`.
- [ ] **Step 2: Verify** `go vet ./... && go test ./...`.
- [ ] **Step 3: Commit** `README.md CLAUDE.md` with message `docs: lazygit-style git wt app and lazygit integration`.
