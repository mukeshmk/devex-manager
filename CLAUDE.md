# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

DevEx Manager is a single Go binary distributed via `install.sh` and GitHub Releases. It ships four tool families:

- `git wt` — a `git worktree` wrapper for bare-repo based multi-worktree workflows, with a full-screen app.
- `dx nb` — Jupyter notebook hygiene utilities (strip outputs, kernel registration, diff, list).
- `dx ctx` — per-branch scratchpad/todo notes stored outside of git tracking.
- `dx skills` — bidirectional sync of AI-agent skill folders (Claude, Kiro, Antigravity, etc.) against a master directory, with a full-screen grid.

Plus optional Python auto-venv shell hooks (`shell/auto-venv/`) and git aliases (`shell/git-aliases/`), both installed by `install.sh`.

## Git rule for this repo

Personal repo: never run host `git` here. Commit via the `personal-git:v3` container (see the `personal-git` skill). Never push or tag; the owner does.

## Building and testing

```bash
go vet ./... && go test ./...                      # from repo root
go build -o dist/dx ./cmd/dx && ln -sf dx dist/git-wt   # build output goes to dist/ only (gitignored)
bash shell/auto-venv/test_auto_venv_prompt.sh     # only Bash test (auto-venv)
```

Go tests use temp git repos (`internal/shared/testutil`) with an isolated git env (`GIT_CONFIG_GLOBAL=/dev/null`, fixed author/committer). CLI tests call `cli.Run(argv0, args, in, out, errw)` with a non-TTY stdin and `bytes.Buffer`s. No linter beyond `go vet`.

## Architecture

One binary, two names: `~/.local/bin/dx`, and `git-wt` a symlink to it (found by git's subcommand discovery as `git wt`). `cmd/dx/main.go` sets `cli.Version` and calls `cli.Main`, which dispatches on `argv[0]`. Unrecognised `git wt` verbs (`lock`, `move`, `prune`, `repair`, `unlock`; `list` with args) pass through to native `git worktree`; `git wt` with no args or `status` opens the full-screen app on a TTY. Non-TTY never opens a full-screen app and never blocks: `git wt sync` takes each question's default without reading stdin; other prompts read stdin and EOF means "no".

Layout, grouped by tool (folder names differ from some package names, kept short for readable call sites):

```
cmd/dx/                     main: argv[0] dispatch
internal/cli/               cobra command wiring for every tool; one file per subcommand (wt_*.go, dx_*.go)
internal/gitwt/worktree/    package wt      — add, clone, rm, clean, status, deletion safety (safety.go)
internal/gitwt/sync/        package wtsync  — git wt sync: scan, plan, apply (with backups), manifest
internal/gitwt/flows/       package flows   — sync/clean/rm flows shared by the CLI and the app
internal/gitwt/app/         package wtapp   — full-screen worktree app (includes the dx ctx todo pane)
internal/skills/            package skills  — skill scan, plan, apply, backups, diff/rm/edit ops
internal/skills/app/        package skillsapp — full-screen skills grid
internal/ctx/               dx ctx notes/todos file handling
internal/nb/                dx nb strip/kernel/diff/list
internal/shared/config/     .devex.conf INI parsing: repo config and global [skills] config with DX_* env overrides
internal/shared/gitx/       shells out to the git CLI (never a Go git lib)
internal/shared/ui/         width-aware tables, shared Fit/Box helpers, prompts, TTY detection
internal/shared/testutil/   temp git repo helpers for tests
shell/auto-venv/            auto-venv.sh hook + its Bash test
shell/git-aliases/          install.sh / uninstall.sh for the git aliases
shell/completions/          git-wt.bash (git's completion needs a _git_wt function)
skills/git-wt/              agent skill (SKILL.md) that install.sh copies to ~/.claude/skills/git-wt
docs/                       workflows.md, specs, plans
```

Errors are printed verbatim to stderr (no extra `Error:` prefix).

In-app actions in `internal/gitwt/app` must call `flows` and never duplicate safety logic. Charm stack: lipgloss, huh, bubbletea, bubbles (keep current majors).

### Config resolution order

CLI arguments > `.devex.conf` (repo-root for `git wt`, `~/.devex.conf` `[skills]` for `dx skills`; `DX_*` env vars override the file) > hardcoded defaults in `internal/config`.

### State storage locations

- `git wt sync` manifest: `$(git rev-parse --git-common-dir)/info/devex/sync-manifest`.
- `git wt sync` backups (before any overwrite/delete): `<common>/info/devex/sync-backups/YYYY-MM-DD-HHMMSS/<worktree>/<rel-path>`.
- `dx ctx` notes: `<common>/info/devex/contexts/<branch-with-slashes-as-__>.md`.
- `dx skills` backups: timestamped copies under `<master_dir>/.backups/YYYY-MM-DD-HHMMSS/` before any overwrite/delete.

`<common>` is always the absolute git common dir, shared across worktrees and never committed. Never use `.git` directly; repos here are typically bare + multi-worktree.

### Worktree deletion safety

`git wt clean` and `git wt rm` check that configured copy paths are in sync across worktrees before deleting anything; if not, they abort and point at `git wt sync` They also refuse dirty worktrees (uncommitted changes (untracked files at or under a configured copy path, and configured symlinks that are real symlinks, do not count), or commits that are on no remote, unless the branch is merged into the base branch or its upstream branch was deleted) unless ticked in `clean` or `--force` for `rm`; `rm --force` skips both the sync check and the dirty refusal. `rm` resolves its target as exact path, then folder name, then branch, and never falls back to a prefix match. The app's delete can stash or discard uncommitted changes (`flows.Dirty`) when nothing else is at risk, but never overrides commits on no remote. Never delete the current worktree, the main worktree or the base branch. Don't bypass these checks when modifying deletion paths.

### Style conventions

- Plain `go vet`-clean Go; shell out to `git` via `internal/shared/gitx`.
- Sync plans are computed in full before anything is touched; `git wt sync --dry-run` prints the plan; `--yes/-y` accepts it (`sync`, `clean`, `dx skills sync`).
- Copies preserve file mode and mtime.
