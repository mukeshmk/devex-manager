---
name: git-wt
description: How to create, use, sync and remove git worktrees in repos managed by DevEx Manager's `git wt` (bare repo + one folder per branch, configured by `.devex.conf`). Use this whenever you are about to create a worktree, start work on a new branch in isolation, switch to another branch's checkout, sync config files between worktrees, or delete a worktree, and whenever you see a `.devex.conf` file or a repo whose root holds a bare `.git` plus a `main/` folder. It overrides any other worktree skill or habit (such as `superpowers:using-git-worktrees` or a `.worktrees/` folder) in these repos.
---

# Working in `git wt` repos

`git wt` (DevEx Manager) manages a bare repo with one folder per branch. Plain
`git worktree add` and agent-chosen locations break that layout: the folder lands
in the wrong place, gets the wrong name, and misses the symlinks and copied config
files the owner relies on. In these repos, every worktree change goes through
`git wt`.

## Is this a `git wt` repo?

Yes if either holds:

- `.devex.conf` exists at the repo root (the folder that holds the bare `.git`).
- The repo root holds a bare `.git` plus worktree folders such as `main/`.

The layout looks like this:

```
<repo-root>/
├── .git/            bare repo (never edit or work in here)
├── .devex.conf      optional config
├── main/            main worktree (base branch)
├── PROJ-123/        one folder per branch
└── ...
```

To find the root from inside any worktree, run `dirname "$(git rev-parse --path-format=absolute --git-common-dir)"`.

If the repo isn't one of these, `git wt` doesn't apply. Use whatever the user or
project normally does.

## Read `.devex.conf` first

Before you create a worktree or name a branch, read `<repo-root>/.devex.conf` if it exists.
It's INI. These are the keys and their defaults:

| Section / key | Default | Meaning |
|---|---|---|
| `[repo] base_branch` | `main` | new branches are created from this |
| `[repo] main_worktree_name` | `main` | folder of the main worktree; source of symlinks/copies |
| `[worktree] naming_strategy` | `ticket-prefix` | how the folder name is derived from the branch |
| `[symlinks] paths` | `.claude,.kiro,.vscode` | paths symlinked from the main worktree into each new one |
| `[copies] paths` | (none) | paths copied from the main worktree into each new one, kept in step with `git wt sync` |

Naming strategies:

- `ticket-prefix`: the folder is the first two `-`-separated parts of the branch.
  `PROJ-123-fix-login` → `PROJ-123/`. Start branch names with the ticket ID
  (ask the user for it if you don't know it). Otherwise unrelated branches collide:
  `fix-login-bug` and `fix-login-ui` both map to `fix-login/`.
- `full-branch`: the folder is the branch with `/` replaced by `--`. `feat/login` → `feat--login/`.
- `custom`: you must pass a folder name: `git wt add <branch> <folder>`. Ask the user which name they want.

Don't edit `.devex.conf` unless the user asks you to; it's the owner's config.

## Create a worktree

```bash
cd <repo-root>        # or any worktree in the repo
git wt add <branch>   # add <folder> only with naming_strategy = custom
cd <repo-root>/<folder>   # git wt add prints this exact path at the end
```

`git wt add` checks out the branch if it already exists locally or on `origin`.
Otherwise it creates the branch from `base_branch`. It puts the folder at
`<repo-root>/<folder>`, then sets up the symlinks and copies from the main worktree.
Don't pass a path. Don't run `git worktree add`, `git checkout -b` in an existing
worktree, or a worktree-creating skill. The command needs a branch name: without
a terminal (the agent's case) it refuses rather than opening the branch picker.

If the target folder already exists, someone already has a worktree for that name.
`cd` into it and check `git branch --show-current` instead of creating another.

Do all your work inside the worktree folder: edits, builds, tests, commits. Never
work in `<repo-root>` or `.git`.

## Look around

- `git wt status` (alias `git wt list`): table of all worktrees with branch, dirty
  state and sync state. With no terminal it prints the table and never opens the full-screen app.
- `git wt list <args>` and the native verbs `lock`, `move`, `prune`, `repair` and
  `unlock` pass straight through to `git worktree`. Using them is fine.

## Keep copied config in step

Files under `[copies] paths` drift between worktrees. If you changed one, or a
removal was refused because copies aren't in sync:

```bash
git wt sync --check     # list what is out of sync
git wt sync --dry-run   # show the full plan
```

Show the user the plan and let them decide. Don't run `git wt sync --yes` on your
own: it can overwrite another worktree's version of a file. It backs files up
first, but which version wins is the user's call. Without a terminal, plain
`git wt sync` takes each question's default and doesn't wait for input.

## Remove worktrees

```bash
git wt rm <folder-or-branch>   # exact path, then folder name, then branch; no prefix match
git wt clean                   # stale/merged worktrees; interactive
```

These commands refuse to delete a worktree in two cases: copy paths aren't in
sync, or the worktree has unsaved work (uncommitted changes, or commits that exist
on no remote). If either happens, stop and tell the user what the message says,
for example "run `git wt sync` first" or "the worktree has uncommitted changes".
Don't route around it:

- Don't add `--force` to `git wt rm` or `--yes` to `git wt clean` unless the user explicitly asked for that.
- Don't run `rm -rf` on a worktree folder or `git worktree remove`.
- Never remove the main worktree, the worktree you're in, or the base branch.

Removing is the user's decision. When you finish a task, leave the worktree in
place unless they asked you to clean up.

## Prompts without a terminal

Your shell has no TTY. `git wt` never opens its full-screen app there. Most
prompts read stdin, and EOF counts as "no", so a command that needs confirmation
does nothing. If that happens, tell the user and suggest they run the command
themselves, for example with `! git wt clean` in Claude Code. Don't pipe `yes`
into it.

## Related

- `dx ctx`: per-branch notes and todos stored outside git (`dx ctx show`). Use it
  if the user keeps notes there.
- New clone in this layout: `git wt clone <url>` creates `<name>/.git` (bare) plus the main worktree.
