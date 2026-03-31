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

The quickest way to install DevEx Manager is via the remote installer. This will download the necessary scripts and configure your shell automatically.

### Remote Installation (Recommended)

```bash
bash -c "$(curl -fsSL https://raw.githubusercontent.com/mukeshmk/devex-manager/main/install.sh)"
```

### Local Installation

If you prefer to audit the code first or have already cloned the repository:

```bash
git clone https://github.com/mukeshmk/devex-manager.git
cd devex-manager
bash install.sh
```

### What the installer does:
1. Downloads/Copies `git-wt` and the sub-command scripts to `~/.local/bin/`
2. Makes all scripts executable
3. **Optional:** Prompts to install a set of handy [Git aliases](#optional-git-aliases)
4. **Optional:** Prompts to install [Python auto-venv tools](#python-auto-venv)
5. Sets up shell auto-completion for `git wt` commands
6. Adds `~/.local/bin` to your `PATH` in `~/.zshrc` or `~/.bashrc`

After installing, open a new terminal or run `source ~/.zshrc` (or your respective shell config).

## Uninstallation

To completely remove DevEx Manager and its configurations:

### Remote Uninstallation

```bash
bash -c "$(curl -fsSL https://raw.githubusercontent.com/mukeshmk/devex-manager/main/uninstall.sh)"
```

### Local Uninstallation

```bash
bash uninstall.sh
```

The uninstaller will:
1. Remove all binaries from `~/.local/bin`
2. Remove the `git-wt-tools` directory
3. **Optional:** Prompts to remove the global Git aliases
4. Clean up the `DevEx Manager` blocks from your shell configuration files (a backup is created as `~/.zshrc.devex-backup`)


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
├── .devex.conf    # per-repo configuration file
└── main/          # worktree checked out to the default branch
```

Steps performed:
1. Creates a directory named after the repo
2. Clones the repo as a bare repository (`.git`)
3. Configures `remote.origin.fetch` for proper branch tracking
4. Fetches all remote refs
5. Creates a `main` worktree
6. Generates a `.devex.conf` configuration file with sensible defaults

If a `.devex.conf` already exists (e.g., re-clone scenario), it is preserved and a warning is printed.

### `git wt add <branch-name> [custom-folder-name]`

Creates a new worktree (or checks out an existing branch) using a short folder name derived from the branch.

```bash
# Create a new branch from the configured base branch
git wt add JIRA-1234-add-login-page

# Use a custom folder name (requires naming_strategy = custom in .devex.conf)
git wt add JIRA-1234-add-login-page login-page
```

| Parameter | Default | Description |
|---|---|---|
| `branch-name` | *(required)* | Full branch name (e.g. `JIRA-1234-add-login-page`) |
| `custom-folder-name` | *(none)* | Custom worktree folder name (required when `naming_strategy = custom`) |

The base branch, main worktree name, and naming strategy are all read from `.devex.conf` (see [Configuration](#configuration)).

The folder name is derived using the configured naming strategy (see [Configuration](#configuration) below). By default, the `ticket-prefix` strategy extracts the first two hyphen-separated segments of the branch name. For example, `JIRA-1234-add-login-page` becomes `JIRA-1234/`.

After creating the worktree, it symlinks the directories listed in the config (default: `.claude`, `.kiro`, `.vscode`) from the main worktree into the new one so your dev environment is ready to go. CLI arguments always take precedence over config values.

Resulting structure:

```
my-project/
├── .git/
├── .devex.conf     # configuration file
├── main/           # your main worktree
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
3. Removes symlinks for configured directories (from `.devex.conf` or defaults)
4. Removes the worktree via `git worktree remove --force`
5. Deletes the local branch via `git branch -D`

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

## Configuration

`git wt clone` generates a `.devex.conf` file at the root of the bare repository. This file controls how worktrees are created and cleaned up. Edit it to customize behavior per repository.

### Default `.devex.conf`

```ini
# DevEx Manager Configuration
# This file was generated by 'git wt clone'.
# Edit the values below to customize worktree behavior for this repository.

[repo]
# The default base branch for creating new worktrees.
base_branch = main

# The name of the primary worktree directory.
main_worktree_name = main

[symlinks]
# Comma-separated list of files and directories to symlink from the main worktree
# into each new worktree. Paths are relative to the worktree root.
# Symlinked paths share the same underlying file — changes in one worktree
# are reflected in all others.
paths = .claude,.kiro,.vscode

[copies]
# Comma-separated list of files and directories to copy from the main worktree
# into each new worktree. Paths are relative to the worktree root.
# Copied paths are independent — each worktree gets its own copy that can
# diverge from the original.
paths =

[worktree]
# How to derive the worktree folder name from the branch name.
# Options:
#   ticket-prefix  - Use first two hyphen-delimited segments (e.g., JIRA-1234)
#   full-branch    - Use the full branch name (/ replaced with --)
#   custom         - Use a custom folder name passed as the last argument to 'git wt add'
naming_strategy = ticket-prefix
```

### Configuration Options

| Section | Key | Default | Description |
|---|---|---|---|
| `[repo]` | `base_branch` | `main` | Default base branch for creating new worktrees |
| `[repo]` | `main_worktree_name` | `main` | Name of the primary worktree directory |
| `[symlinks]` | `paths` | `.claude,.kiro,.vscode` | Comma-separated list of files and directories to symlink from the main worktree into new worktrees (shared — changes reflect everywhere) |
| `[copies]` | `paths` | *(empty)* | Comma-separated list of files and directories to copy from the main worktree into new worktrees (independent — each worktree gets its own copy) |
| `[worktree]` | `naming_strategy` | `ticket-prefix` | How to derive folder names from branch names |

### Naming Strategies

| Strategy | Behavior | Example |
|---|---|---|
| `ticket-prefix` | Extracts the first two hyphen-delimited segments | `JIRA-1234-add-login` → `JIRA-1234` |
| `full-branch` | Uses the full branch name, replacing `/` with `--` | `feature/auth/login` → `feature--auth--login` |
| `custom` | Uses the custom folder name passed as the last argument to `git wt add` | `git wt add JIRA-123-feat my-folder` → `my-folder` |

### Precedence

Configuration values are resolved in this order (highest priority first):

1. CLI arguments (e.g., the `[main-worktree-name]` parameter)
2. Values from `.devex.conf`
3. Hardcoded defaults

When no `.devex.conf` exists, all commands behave identically to before — full backward compatibility is preserved.

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

During installation you'll be prompted to install shortcut aliases via the `git-aliases/install-git-aliases.sh` module. These are set via `git config --global` and include:

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
