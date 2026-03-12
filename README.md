# DevEx Manager — Developer Experience Tools

A collection of lightweight CLI tools to enhance your development workflow. DevEx Manager includes `git wt` for managing Git worktrees and Python auto-venv tools for seamless virtual environment management.

## git-wt — Git Worktree Manager

A lightweight CLI tool that wraps `git worktree` to simplify working with bare repositories and multiple worktrees. Instead of juggling branches in a single checkout, `git wt` lets you clone repos as bare, spin up worktrees as folders, and tear them down cleanly — all with short, memorable commands.

## Why?

Git worktrees let you have multiple branches checked out simultaneously in separate directories. This is great for:

- Reviewing a PR while keeping your current work untouched
- Running tests on one branch while developing on another
- Avoiding constant `git stash` / `git checkout` cycles

`git wt` removes the boilerplate and adds smart defaults on top of the native `git worktree` commands.

## Installation

```bash
git clone https://github.com/mukeshmk/devex-manager.git
cd devex-manager
bash install.sh
```

The installer:
1. Copies `git-wt` and the sub-command scripts to `~/.local/bin/`
2. Makes all scripts executable
3. Optionally installs a set of handy Git aliases (see Optional Git Aliases section below)
4. Optionally installs Python auto-venv tools (see Python Auto-Venv section below)
5. Sets up shell auto-completion for `git wt` commands
6. Adds `~/.local/bin` to your `PATH` in `~/.zshrc` or `~/.bash_profile` / `~/.bashrc` (if not already present)

After installing, open a new terminal or run:

```bash
source ~/.zshrc   # or source ~/.bash_profile
```

## Commands

### `git wt clone <url>`

Clones a repository as a bare repo and sets up an initial `main` worktree.

```bash
git wt clone git@github.com:user/my-project.git
```

This creates the following structure:

```
my-project/
├── .git/          # bare repository
└── main/          # worktree checked out to the default branch
```

Steps performed:
1. Creates a directory named after the repo
2. Clones the repo as a bare repository (`.git`)
3. Configures `remote.origin.fetch` for proper branch tracking
4. Fetches all remote refs
5. Creates a `main` worktree

### `git wt add <branch-name> [base-branch] [main-worktree-name]`

Creates a new worktree (or checks out an existing branch) using a short folder name derived from the branch.

```bash
# Create a new branch from main
git wt add JIRA-1234-add-login-page

# Create from a specific base branch
git wt add JIRA-1234-add-login-page develop

# Specify a custom main worktree name
git wt add JIRA-1234-add-login-page main master
```

| Parameter | Default | Description |
|---|---|---|
| `branch-name` | *(required)* | Full branch name (e.g. `JIRA-1234-add-login-page`) |
| `base-branch` | `main` | Branch to create the new branch from |
| `main-worktree-name` | `main` | Name of the main worktree folder (for copying config) |

The folder name is automatically derived from the first two hyphen-separated segments of the branch name. For example, `JIRA-1234-add-login-page` becomes `JIRA-1234/`.

After creating the worktree, it copies IDE/editor configuration directories (`.claude`, `.kiro`, `.vscode`) from the main worktree into the new one so your dev environment is ready to go.

Resulting structure:

```
my-project/
├── .git/
├── main/          # your main worktree
└── JIRA-1234/      # new worktree for the feature branch
```

### `git wt rm <folder-name-or-branch-name>`

Safely removes a worktree and deletes its local branch.

```bash
# Remove by folder name
git wt rm JIRA-1234

# Remove by branch name
git wt rm JIRA-1234-add-login-page
```

Steps performed:
1. Finds the worktree by matching the folder name or branch name
2. Prevents deletion if you're currently inside the target worktree
3. Removes the worktree via `git worktree remove --force`
4. Deletes the local branch via `git branch -D`

If the worktree folder was already manually deleted, the command still attempts to clean up the local branch.

### Native Git Worktree Commands

`git wt` also passes through native git worktree commands for convenience:

```bash
git wt list      # List details of each worktree
git wt lock      # Prevent a worktree from being pruned
git wt move      # Move a worktree to a new location
git wt prune     # Prune working tree information
git wt repair    # Repair worktree administrative files
git wt unlock    # Unlock a worktree
```

These commands are passed directly to `git worktree`, so you can use all the same options and flags.

## Python Auto-Venv

The installer optionally includes Python virtual environment automation that:

- Automatically activates `.venv` when you `cd` into a project directory
- Automatically deactivates when you leave the project
- Provides a quick `venv` command to create new virtual environments using `uv`

### Features

- Searches up the directory tree to find `.venv/bin/activate`
- Seamlessly switches between different project venvs
- Works with any shell that supports bash-style functions

### Usage

Once installed, the auto-venv tools work automatically:

```bash
# Navigate into a project with a .venv
cd my-python-project
# → .venv automatically activated

# Leave the project
cd ~
# → .venv automatically deactivated

# Create a new venv quickly (requires uv)
cd new-project
venv
# → Creates .venv using uv and activates it
```

### Requirements

- The `venv` command requires [uv](https://github.com/astral-sh/uv) to be installed
- Auto-activation works with any `.venv` created by any tool

## Optional Git Aliases

During installation you'll be prompted to install shortcut aliases. These are set via `git config --global` and include:

| Alias | Expands To | Description |
|---|---|---|
| `git a` | `git add` | Stage files |
| `git s` | `git status` | Show status |
| `git d` | `git diff` | Show diff |
| `git f` | `git fetch` | Fetch refs |
| `git m` | `git merge` | Merge |
| `git c` | `git checkout` | Checkout |
| `git b` | `git branch` | Branch operations |
| `git l` | `git log` | View log |
| `git r` | `git restore` | Restore files |
| `git rs` | `git restore --staged` | Unstage files |
| `git ls` | `git log --stat` | Log with file stats |
| `git bn` | `git rev-parse --abbrev-ref HEAD` | Print current branch name |
| `git fet` | `git fetch origin <current-branch>` | Fetch current branch |
| `git mer` | `git merge @{u}` | Merge upstream into current branch |
| `git pul` | `git pull origin <current-branch>` | Pull current branch |
| `git pus` | `git push origin <current-branch>` | Push current branch |
| `git stas` | `git stash push -p` | Interactive stash |

## Typical Workflow

```bash
# 1. Clone a repo as a bare worktree setup
git wt clone git@github.com:user/my-project.git
cd my-project/main

# 2. Start a new feature
git wt add JIRA-1234-add-login-page
cd ../JIRA-1234

# 3. Do your work, commit, push...

# 4. Clean up when done
cd ../main
git wt rm JIRA-1234
```

## Requirements

- Git 2.15+ (worktree support)
- Bash 4+
- macOS or Linux

## License

MIT
