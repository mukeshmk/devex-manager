# DevEx Manager v2: Go + TUI rewrite

Date: 2026-10-30
Status: draft, awaiting approval

## Why

The owner works on one laptop that moves between three screens: the laptop panel, an office monitor and a home monitor. Today every table in the tools (`git wt status`, `dx skills list`, `dx nb list`) is printed with hardcoded `printf` widths and fixed-length dash dividers, so output wraps on the laptop and leaves big gaps on the monitors. Long branch names and ANSI colour codes push columns out of alignment.

`git wt sync` also feels slow. Measured causes in the current Bash (`git-wt-tools/git-wt-sync`):

1. Conflict-marker detection runs `find` over the **whole main worktree**, pruning only `.git` and symlinked paths, so it walks `node_modules`, `.venv`, build output, etc.
2. Per file × per worktree it forks `stat`, plus `diff` and `basename`: hundreds of processes for modest setups.
3. `git wt clean` and `git wt rm` each run the full sync check first; `clean` then runs `git ls-remote` once per worktree (a network round-trip each).

And the code has grown hard to change: `git-wt-sync` is 666 lines with the "copy to main then fan out" block pasted ~10 times, `git-wt-clean` duplicates its whole delete block for the "all" vs "individual" paths, and each tool family has its own INI parser.

## Goals

- Every view fits the current terminal width and re-lays itself out live on resize.
- `git wt sync` and the pre-delete sync check feel instant on a normal repo.
- Clean, small codebase with one implementation of each concept (config, git calls, file sync, tables).
- Better interaction: arrow-key selection and checkboxes instead of typed `y/n/individual` / `[1-3]` prompts.

## Non-goals

- New features beyond what is listed here.
- Changing the `.devex.conf` / `~/.devex.conf` format, key names, defaults, or state file locations.
- Windows support (macOS arm64/amd64 and Linux amd64/arm64 only).
- Rewriting the shell-side pieces: `auto-venv/auto-venv.sh`, git aliases, and the shell-rc block stay shell.
- Replacing the external notebook tools (`nbstripout`, `nbconvert`, `nbdime`, `ipykernel`). `dx nb` keeps calling them.

## Anti-goals (would make this a failure even if it works)

- **Sync ever loses or silently overwrites a config file.**
- **`git wt clean` or `git wt rm` deletes anything the user didn't see listed and confirm**, or discards uncommitted or unpushed work without an explicit opt-in.

## Decisions made with the owner

- **Language/libraries:** Go, using the Charm stack (Bubble Tea for full-screen apps, Lip Gloss for styling and width-aware tables, Huh or Bubble Tea components for pickers/confirms). Chosen by Claude, accepted by the owner. Go is installed on the dev laptop (go1.27.3 darwin/arm64).
- **Full-screen apps for two tools:**
  - `git wt` (no args) and `git wt status`: full-screen. Left pane is the worktree list (name, changes, ahead/behind, merged). Right pane is details for the selected worktree: branch, path, remote state, changed files, and that branch's `dx ctx` todos. Single-key actions: move ↑↓, sync `s`, delete `d`, clean merged `c`, filter `/`, quit `q`. Approved from the mockup in `.superpowers/brainstorm/` ("Option B"), **minus "Enter = cd into worktree"**, which is dropped for v2 (may come back later as a separate feature). Enter does nothing special; the selected worktree's path is shown in the details pane.
  - `dx skills` (no args) and `dx skills list`: full-screen grid of skills (rows) × master + each agent (columns), marking which copy is newest/different, with a diff of the selected skill in a side pane. Sync is picked from the grid instead of one confirm for everything.
- **`dx ctx`:** no screen of its own. Its notes/todos show in the `git wt` details pane (toggle a todo with a key). All existing `dx ctx` subcommands stay as print-and-exit.
- **`dx nb`:** stays print-and-exit (`strip`, `kernel`, `diff`, `list`). `list` uses the shared width-aware table.
- **Every other subcommand stays print-and-exit** (`git wt add/clone/rm/sync/clean`, `dx skills sync/diff/edit/rm`, all `dx ctx` verbs), so scripts and muscle memory keep working. Interactive prompts inside them become arrow-key pickers when stdin is a TTY, and keep today's non-interactive defaults when it is not.
- **Safety (owner-approved):**
  - Sync keeps today's rules exactly (main is the hub; changes fan out; conflicts are never resolved silently; `.devex-conflict-*` files when deferred; manifest at `$(git rev-parse --git-common-dir)/info/devex/sync-manifest`).
  - Sync computes a full **plan** before touching anything, shows it, and asks to confirm. `git wt sync --dry-run` prints the plan and exits.
  - **New:** before sync overwrites or deletes any file, it copies the old version to a timestamped backup dir under `$(git rev-parse --git-common-dir)/info/devex/sync-backups/YYYY-MM-DD-HHMMSS/<worktree>/<rel-path>`.
  - `clean` and `rm` show exactly what will be deleted (folder, branch) and what would be lost (uncommitted files, commits not on any remote) before confirming.
  - **New: dirty worktrees are refused by default.** A dirty worktree is not selected in `clean` and is refused by `rm`, unless the user explicitly ticks it (`clean`) or passes `--force` (`rm`). Definitions (owner-approved):
    - **Dirty** if it has uncommitted changes (`git status --porcelain` non-empty), **or** its branch never had an upstream (no `branch.<name>.remote` config) and has commits not reachable from any remote-tracking ref.
    - **Otherwise safe**, i.e. any of: merged into base; had an upstream whose remote branch is now gone (the squash-merge-then-delete case, checked with the live `ls-remote` result `clean` already fetches); or all commits are on a remote.
    - This also fixes a current bug: today `clean` treats a never-pushed branch as "remote branch deleted" and offers it for deletion.
  - Kept: never delete the current worktree, the main worktree or the base branch; never delete while copy paths are out of sync (`rm --force` keeps its current meaning of skipping this check).
- **Git:** this is a personal repo. All git operations go through the `personal-git:v3` container (see the `personal-git` skill), never host git. The agent never pushes; the owner pushes commits and release tags.
- **Distribution:** a GitHub Actions workflow builds release binaries when a `v*` tag is pushed and attaches them to a GitHub Release. `install.sh` stays a single `curl | bash` and downloads the right binary for the machine. Users never need Go.

## Constraints

- Behaviour parity: everything the Bash tools do today keeps working with the same commands, flags, config and state files, except the deliberate safety changes above. The existing scripts are the behaviour spec, especially `git-wt-tools/git-wt-sync` (all conflict branches), `git-wt-add` (naming strategies, symlink + copy steps, fzf branch picker when no arg), `git-wt-rm` (target matching by folder suffix or branch, branch-prefix fallback), `dx-skills-tools/*` (env var overrides `DX_*`, per-tool `*_sync_only`, global `sync_only`/`ignore`, `.backups/` in master dir), `dx-ctx-tools/*` (file naming `<branch with / as __>.md`, todo index semantics).
- `git wt` must still be found by git's subcommand discovery (an executable named `git-wt` on `PATH`), and `dx` must be an executable named `dx`. Unrecognised `git wt` verbs (`lock`, `move`, `prune`, `repair`, `unlock`, and `list` with args) still pass straight through to `git worktree`.
- Cross-worktree-aware: use the git common dir, never `.git` directly. Repos are typically bare + multi-worktree.
- Non-TTY use (pipes, scripts, tests) must never open a full-screen app or block on a prompt. Fall back to plain table output and today's defaults.
- Existing tests in `tests/` are the parity oracle. Port their scenarios to Go tests before deleting the scripts they cover.

## Calls made by Claude (owner can override)

- **One binary, two names.** A single Go binary installed as `~/.local/bin/dx`, with `~/.local/bin/git-wt` a symlink to it; it dispatches on `argv[0]`. Avoids two builds and shared-code duplication.
- **Sync speed approach:** stat files directly in-process (no forks); scan only the configured copy paths for `.devex-conflict-*` markers instead of the whole worktree; check worktrees concurrently; compare contents only when size/mtime say a file might have changed. `clean` resolves remote-branch existence with one `git ls-remote --heads <remote>` per remote, not per worktree.
- **Git access:** shell out to the `git` CLI rather than a Go git library, so behaviour (bare repos, worktrees, credentials) matches the user's git exactly.
- **Completions:** generate bash/zsh completions from the binary (`dx completion zsh` etc.) and source them from the rc block, replacing `git-wt-completion.sh` / `dx-completion.sh`.
- **Upgrade path:** `install.sh` removes the old `*-tools/` script directories and old router scripts from `~/.local/bin` before installing the binary. `uninstall.sh` is updated to match. Config, manifests, ctx notes and backups are untouched.
- **Build order** (each stage leaves a working tool; Bash scripts stay in the repo until their Go replacement passes the ported tests):
  1. Foundation: module layout, config loading, git helpers, width-aware table, `argv[0]` dispatch.
  2. `git wt` print commands: `add`, `clone`, `rm`, `sync` (with plan/dry-run/backups), `clean`, plain `status` for non-TTY.
  3. `git wt` full-screen app (including `dx ctx` pane).
  4. `dx ctx` and `dx nb` commands.
  5. `dx skills` commands and full-screen grid.
  6. Release workflow, new `install.sh`/`uninstall.sh`, README/CLAUDE.md update, delete the Bash tools and old tests.
- **No-terminal behaviour (pipes, scripts, tests):** prompts read their answers from stdin exactly as today, so `echo y | git wt clean` still works. If stdin has no answer (EOF), destructive steps default to **no** and abort. `--yes/-y` on `clean`, `sync` and `dx skills sync` accepts the plan without asking. `rm <target>` with an explicit target counts as the confirmation (as today), but still refuses dirty worktrees without `--force`. `sync` with no terminal applies today's defaults (conflicts left as "resolve later", which writes `.devex-conflict-*` files and never overwrites), with backups taken as normal.
- **`dx skills` grid scope:** you tick or untick the pending updates the current rule finds (newest copy by mtime wins, respecting `sync_only`/`ignore`). Choosing a different source copy is out of scope for v2.
- **Install from a local clone:** if `install.sh` runs inside a clone and `go` is on `PATH`, it builds the binary with `go build` (the dev/testing path for unreleased changes). Otherwise it downloads the latest GitHub Release from `mukeshmk/devex-manager`. No version pinning in v2.
- **Shells:** the rc block and completions support both zsh and bash, as `install.sh` does today.
- **Version:** first Go release is `v2.0.0`.
- `.superpowers/` gets added to `.gitignore`.

## Left to the builder

- Go package layout, internal APIs, naming.
- Exact colours, borders, key bindings beyond those listed, and how columns shrink/truncate at narrow widths (with a minimum-width fallback message if the terminal is too small).
- Test structure (table-driven tests with temp git repos are expected; the old `PATH`-mock approach doesn't need to be kept).
- Release tooling choice (e.g. GoReleaser vs a plain matrix `go build` job).

## Known oddity to fix while porting

- `git wt` help text says `sync` supports `--force/-f`, but `git-wt-sync` doesn't implement it. Drop it from the help unless a meaning is agreed.

## Repo housekeeping noted during design

- This repo's local `.git/config` sets `user.name=T` / `user.email=t@e.com`, which overrides the personal identity in the `personal-git` container. Remove these two keys before the first commit of this work (owner did not object).
