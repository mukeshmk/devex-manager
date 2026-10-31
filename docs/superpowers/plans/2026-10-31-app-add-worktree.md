# git wt app: add worktrees and fetch from the UI

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task.

**Goal:** In the `git wt` full-screen app, `n` creates a worktree, either for a remote branch picked from a filterable list or for a new branch whose name you type, and `f` fetches from the remote. Both run in-app like sync, clean and delete: questions are popups, output goes to the command log, nothing drops to the shell.

**Why:** The owner can't see which remote branches exist from the app, and has no key to add a worktree. Today that needs `git wt add` in the shell. Agreed with the owner on 2026-10-31.

**Architecture:** The same pattern as the 2026-10-30 lazygit-style plan (`docs/superpowers/plans/2026-10-30-lazygit-style-app.md`):
- The body of `runAdd`, without the branch picker, moves to `flows.Add`. The CLI keeps its huh picker, and the app uses its own popup.
- `f` runs a `flows.Fetch`.
- The choose popup gains type-to-filter, because kaleidoscope has 81 remote branches.

**Spec:** this file (small feature). It extends `docs/superpowers/specs/2026-10-30-lazygit-style-app-design.md`, whose rules all still apply.

## Decisions

- **`n` (any panel, not busy)** opens a choose popup titled `New worktree`.
  - The first option is `+ new branch…`. The rest are `wt.RemoteBranchChoices`: origin branches that have no worktree, sorted.
  - Typing filters the list (case-insensitive substring). `+ new branch…` always stays first.
  - Picking a remote branch runs `flows.Add(e, branch, "")`.
  - Picking `+ new branch…` opens an input popup `New branch (from <base>)`, and enter runs `flows.Add(e, name, "")`. The flow creates the branch from the base branch when it doesn't exist, exactly as `git wt add <name>` does.
  - If `naming_strategy = custom`, a second input popup `Folder name for <branch>` comes before the flow runs, and its value is passed as the folder.
  - Afterwards the status shows `add: done · <last line>` and the rows reload. The new worktree is selected if it's found, otherwise the selection is kept.
- **`f` (any panel, not busy)** runs `flows.Fetch(e)`, which is `git fetch --all --prune` in the repo root. Afterwards the status shows `fetch: done` and the rows reload (remote/ahead-behind columns change).
- **Fetch and add must never read the terminal**, because Bubble Tea owns it:
  - Git runs with `GIT_TERMINAL_PROMPT=0`.
  - Git runs in a new session (`SysProcAttr{Setsid: true}`, darwin and linux), so ssh can't open `/dev/tty` to ask for a passphrase.
  - An auth failure becomes a red status with git's message. It never hangs and never corrupts the screen.
  - This lives in one gitx function that still calls the command hook, so every git command it runs lands in the command log.
- **CLI unchanged:** `git wt add` output, flags, exit codes and the huh remote-branch picker stay identical. There is no `git wt fetch` command (it would duplicate `git fetch`).
- **Keys:** `n` and `f` go in `helpItems` (help popup and hint line) and in the README key table. `n` and `f` must not clash with existing app keys. Check `helpItems` first.

## Non-goals

- Switching or cd'ing into the new worktree. The app still can't change the parent shell's directory.
- Renaming or moving worktrees.
- Choosing a base branch other than the configured one.
- Fetching a single remote.

## Global Constraints

The 2026-10-30 plan's Global Constraints apply unchanged:
- Never run host git against this repo. Use the personal-git:v3 container.
- No Co-Authored-By lines in commits, and never push.
- Folder names differ from package names: `internal/gitwt/app` is `wtapp`, and the new code goes in `internal/gitwt/flows`.
- No rendered line may exceed the terminal width at any size of 60×12 or more.
- Every internal/cli test passes untouched.
- In-app actions call `flows` and never duplicate logic.

---

### Task 1: flows.Add and flows.Fetch

**Files:**
- Create `internal/gitwt/flows/add.go` and `fetch.go`.
- Modify `internal/cli/wt_add.go` (it becomes a wrapper around `flows.Add`) and `internal/shared/gitx/git.go` (add the non-interactive runner).
- Test in `internal/gitwt/flows/flows_test.go` and `internal/shared/gitx/hook_test.go`.

**Interfaces:**
- `func Add(e Env, branch, folder string) error`. This is `runAdd` after the branch is known: open the repo, load config, call `wt.Add(repo, cfg, branch, folder, e.Out)`. Errors are printed red to `e.Err` and the function returns `ErrReported`. The messages are identical to today's.
- `func Fetch(e Env) error`. It runs `git fetch --all --prune` in the repo root through the gitx non-interactive runner, writes git's combined output to `e.Out`, and on error prints red to `e.Err` and returns `ErrReported`.
- `func RemoteBranches(cwd string) ([]string, error)`. It is `git branch -r` plus `repo.Worktrees()` passed through `wt.RemoteBranchChoices`, moved out of `pickRemoteBranch`. Both the CLI picker and the app use it.
- In gitx, `func RunQuiet(dir string, args ...string) (string, error)`, or a better name. It behaves like `Run`, but sets `GIT_TERMINAL_PROMPT=0` and `Setsid`, returns stdout and stderr combined on success, and calls the hook.
  - `Setsid` comes from `syscall`, so build tags are needed. Give it a `_unix.go` file and a no-op elsewhere, so the build doesn't break on other platforms.

**Tests:**
- `TestAddFlowCreatesBranchFromBase`: a fixture repo, `flows.Add(e, "feat-x", "")`. Expect a folder whose branch is `feat-x` at base, with configured symlinks created.
- `TestAddFlowExistingRemoteBranch`: a fixture with a remote that has branch `r1`. `Add` checks it out.
- `TestFetchFlowNoRemoteFails`: no remote is configured, or the remote is a bad path. Expect `ErrReported`, a red message in `e.Err`, and no hang.
- `TestFetchFlowPrunesAndFetches`: a bare "remote" fixture with a new branch. After `Fetch`, `RemoteBranches` lists it.
- `TestRunQuietNoPrompt`: `RunQuiet` with `GIT_ASKPASS` unset against `https://127.0.0.1:1/x.git` returns an error quickly and does not hang.
- Every existing `internal/cli` `wt_add` test passes unchanged.

**Commit:** `refactor(wt): shared add/fetch flows; non-interactive git runner`

### Task 2: Filterable choose popup

**Files:**
- Modify `internal/gitwt/app/popup.go`.
- Test in `popup_test.go`.

**Interfaces:**
- `popup` gains `filter string` and `pinned int`. `pinned` is the number of leading options that are never filtered out, and `+ new branch…` uses `pinned = 1`.
- Type-to-filter is enabled per popup with a `filterable bool` field. It is off for existing flow prompts, so their `y`/`n`/`j`/`k` keys keep working.
- When filterable:
  - Printable runes, except while ctrl or alt is held, append to `filter`, and backspace removes the last one.
  - `↑`/`↓` move. `j` and `k` are typed into the filter, not used for movement.
  - `enter` returns the index into the original `options`, not the filtered view.
  - `esc` clears the filter first, and a second `esc` cancels.
- The view shows a `filter: <text>` line and `no matches` when nothing matches. Width and height limits are unchanged.

**Tests:**
- `TestPopupFilterNarrows`: options `["+ new branch…", "DSG-1 a", "DSG-2 b", "XYZ"]`. Type `dsg-2`, enter. Expect `Index == 2`.
- `TestPopupFilterKeepsPinned`: a filter that matches nothing still shows `+ new branch…`, and enter returns 0.
- `TestPopupFilterEscClearsThenCancels`.
- `TestPopupFilterViewFits`: 200 options of 120 characters at 60×12 fit the terminal.
- The existing popup tests pass unchanged.

**Commit:** `feat(wtapp): type-to-filter choose popups`

### Task 3: `n` and `f` in the app, plus docs

**Files:**
- Modify `internal/gitwt/app/app.go` (keys, helpItems, the flow wiring, reselection after add).
- Modify `README.md` (key table, a note on add and fetch) and the README lazygit section. It must match the owner's working config:
  - `promptToReturnFromSubprocess: false`
  - `X` (global) for `git wt`, with `output: terminal`
  - `U` for `dx skills`, with `output: terminal`
  - `Y` for `git wt sync`, with `output: popup`
  - `X` in `remoteBranches` and `localBranches` for `git wt add {{.SelectedRemoteBranch.Name}}` and `git wt add {{.SelectedLocalBranch.Name}}`, with `output: popup`
- Test in `internal/gitwt/app/add_test.go`.

**Behaviour:**
- `n`:
  - Load `flows.RemoteBranches(cwd)` in a tea.Cmd, without blocking Update.
  - Open the filterable choose popup.
  - On pick, run `runFlow(..., "add", ...)`. A `+ new branch…` pick goes through the input popup, plus the folder input popup when `naming_strategy = custom`.
  - An empty name does nothing.
  - Popups opened here are local, not flow prompts. Use the existing local-popup mechanism, `onSubmit`, from the Context panel work.
- `f`: `runFlow(..., "fetch", flows.Fetch)`.
- Both are ignored while busy, which matches the existing rules.
- `flowDoneMsg` for `add`: after the reload, select the row whose path is the new worktree. The flow's last Out line holds `cd "<path>"`, or the path can come from the branch name. Pick whichever is simpler and works.

**Tests:**
- `TestAppAddRemoteBranch`: a fixture with remote branch `r1`. Press `n`, type `r1`, press enter. Expect the folder to exist, `statusMsg` to contain `add: done`, and the selected row to be r1.
- `TestAppAddNewBranch`: press `n`, enter (selects `+ new branch…`), type `brand-new`, enter. Expect the branch to exist at base and the folder to exist.
- `TestAppFetch`: with a bare remote that gets a new branch after the clone, press `f` and wait for flowDone. Then `n` lists the new branch.
- `TestAppAddIgnoredWhileBusy`.

**Commit:** `feat(wtapp): add worktrees and fetch from the app`

## Review Focus

1. Fetch and add can never read the TTY, hang on auth, or corrupt the screen. Check `Setsid` and `GIT_TERMINAL_PROMPT`, and that the hook still logs the commands.
2. The CLI `git wt add` behaviour stays byte-identical, picker included.
3. The filter's index mapping: enter must return the original index, and the pinned option must always be reachable.
4. Width safety with the filter line and long branch names at 60×12.
5. The README keys match `helpItems`, and the lazygit snippet matches the working config.
