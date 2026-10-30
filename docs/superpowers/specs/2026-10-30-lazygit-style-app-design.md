# git wt app: lazygit-style, everything in-app

Date: 2026-10-30
Status: draft, awaiting approval

## Why

In the current `git wt` app, `s` (sync), `c` (clean) and `d` (delete) leave the app. The app hands the terminal to a child `git-wt sync` / `clean` / `rm` process, prints that command's output, then shows `Press Enter to return to git wt…`. The owner wants it to feel like lazygit: actions happen in popups inside the app, there's a command log showing what ran, a status area, and a dedicated `dx ctx` area.

## Goals

- `s`, `c` and `d` run inside the app. Questions appear as popups, output goes to a command log panel, and the app never drops to the shell.
- A lazygit-like layout: numbered, titled panels down the left, a main view on the right for the focused item, a command log under the main view, and key hints on the bottom line.
- A `dx ctx` panel where todos for the selected worktree's branch can be added, toggled, deleted and cleared without leaving the app.
- The same behaviour and safety as the command line. The in-app actions run the exact same code paths as `git wt sync`, `git wt clean` and `git wt rm`.

## Non-goals

- No change to the `dx skills` grid (possible follow-up using the same panel style).
- No new worktree operations (add, rename, switch/cd). Enter still does not cd.
- No change to command-line output, flags, exit codes, safety rules, backups or config.
- No mouse support.

## Anti-goals

- An in-app action that bypasses a safety check the command line enforces (sync check before delete, dirty refusal, main/base/current worktree refusal, backups before overwrite).
- Quitting mid-action leaving a half-applied sync or removal.

## Decisions made with the owner

- Move sync and clean into the app, plus anything else that currently drops out of it (that is delete as well).
- Make it look very similar to lazygit, with a command log, a status area, and a separate area for `dx ctx`.

## Calls made by Claude (owner can override when reviewing)

**Layout.** At 80 columns or more:
```
╭─[1] Status──────────────╮╭─ DEVX-1423 ─────────────────────────────╮
│ api · base main         ││ Branch  feature/DEVX-1423-retry-…        │
│ sync: 2 files out of sync││ Path    ~/work/api/DEVX-1423             │
╰─────────────────────────╯│ Remote  origin · ↑2                      │
╭─[2] Worktrees───────────╮│                                          │
│   main      clean synced ││ Changed files                            │
│ ▸ DEVX-1423 ● 4   ↑2     ││  M internal/webhook/retry.go             │
│   DEVX-1301 clean merged ││  ...                                     │
╰─────────────────────────╯╰──────────────────────────────────────────╯
╭─[3] Context─────────────╮╭─ Command log ───────────────────────────╮
│ [ ] add jitter          ││ git worktree list --porcelain            │
│ [✓] cap max delay       ││ sync: update .env in feat, feat2         │
╰─────────────────────────╯╰──────────────────────────────────────────╯
╭─[4] Sync────────────────╮
│ .env  modified in feat  │
╰─────────────────────────╯
 1-4 panels · ↑↓ move · s sync · c clean · d delete · a add todo · ? keys · q quit
```
- The left column is about 1/3 of the width. Status is 4 lines tall. Worktrees, Context and Sync split the rest, with Worktrees getting the most.
- The right column is the main view, about 70% of the height, above the command log, about 30%.
- Below 80 columns the left column stacks above the main view and the command log shrinks to 3 lines. The existing `Terminal too small (need 60×12)` minimum stays.

**Main view by focused panel.**
- Status: repo summary and the sync check result.
- Worktrees: the selected worktree's details, as today.
- Context: the full ctx file (todos and notes).
- Sync: the plan preview, i.e. what `s` would do.

**Keys, lazygit-style.**
- Navigation:
  - `1`–`4` jump to a panel. `tab` / `shift+tab` and `←`/`→` (`h`/`l`) cycle panels.
  - `↑`/`↓` (`j`/`k`) move. Enter moves focus to the main view so it can scroll, and esc moves back.
  - `/` filters worktrees. `r` refreshes. `?` opens a keybindings popup. `q` / ctrl+c quits.
- Worktree actions (any panel): `s` sync, `c` clean, `d` delete the selected worktree.
- Context actions (Context panel): `a` add a todo (text input popup), `space` toggle, `x` delete with a confirm, `C` clear done todos with a confirm.

**Popups.** Centred and bordered, like lazygit menus.
- Kinds: confirm (enter/`y` yes, esc/`n` no), choose (↑↓ + enter, esc = cancel), checklist (space ticks, enter confirms, esc cancels), text input (enter submits, esc cancels).
- A popup never exceeds the terminal size, and long lines truncate with `…`.

**How actions run.**
- The command-line flows for sync, clean and rm move into one shared package. The command line and the app both call it, giving each its own prompter and output writer.
  - The app's prompter turns each question into a popup and waits for the answer.
  - The app's writer feeds the command log.
- Each flow runs in the background. While it runs:
  - The status line shows `sync running…`.
  - Keys other than the popup's are ignored, and `q` is refused with `wait for sync to finish` so nothing is left half-applied.
  - ctrl+c inside a popup counts as abort: sync aborts before applying anything, the same as the command line.
- When the flow finishes, the app refreshes and the status line shows the result, for example `sync: done · backups in …`, or the error in red.

**Command log.**
- Every git command the tool runs is listed, such as `git worktree remove --force <path>`, along with each flow's output lines.
- It is bounded to the last 500 lines and scrolls when focused (`5`, or `tab` to it).

**What goes away.** The `paused` press-Enter wrapper and the self re-exec (`exec.Cmd{Path: self, Args: ["git-wt", …]}`).

## Left to the builder

- Exact colours, border characters and panel title style. Lazygit uses a highlighted border on the focused panel and a dim one on the others.
- The popup component internals, and whether bubbles' `textinput` and `viewport` are used.
- Truncation and scrolling details within the constraints above.
