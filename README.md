# DevEx Manager — Developer Experience Tools

A collection of lightweight CLI tools to enhance your development workflow. DevEx Manager includes `git wt` for managing Git worktrees, `dx nb` for Jupyter Notebook utilities, `git ctx` for branch-specific developer scratchpads, and Python auto-venv tools for seamless virtual environment management.

## Key Features

- **`git wt` (Worktree Manager):** Simplify working with bare repositories and multiple worktrees.
- **`dx nb` (Notebook Utilities):** Essential tools for AI/ML developers to manage Jupyter Notebooks and kernels.
- **`git ctx` (Context Manager):** Local, untracked todo checklists and scratchpad notes per branch.
- **`dx skills` (Prompt Skill Sync Manager):** Bidirectionally synchronize prompt skill folders across multiple AI tools (Claude Code, Kiro, Antigravity, etc.) using a central Master folder.
- **Python Auto-Venv:** Transparent activation/deactivation of `.venv`, proactive initialization using `uv`, and stale worktree detection.
- **Git Aliases:** High-productivity shortcuts for common Git operations.

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
1. Downloads/Copies `git-wt`, `git-ctx`, `dx` and their subcommand scripts to `~/.local/bin/` and target tool folders.
2. Makes all scripts executable
3. **Optional:** Prompts to install a set of handy [Git aliases](#optional-git-aliases)
4. **Optional:** Prompts to install [Python auto-venv tools](#python-auto-venv)
5. Sets up shell auto-completion for `git wt` and `dx` commands
6. Adds `~/.local/bin` to your `PATH` and configures completions in a managed `# >>> DevEx Manager >>>` block in your `~/.zshrc` or `~/.bashrc`

After installing, open a new terminal or run `source ~/.zshrc` (or your respective shell config).

---

## git-wt — Git Worktree Manager

A lightweight CLI tool that wraps `git worktree` to simplify working with bare repositories and multiple worktrees. Instead of juggling branches in a single checkout, `git wt` lets you clone repos as bare, spin up worktrees as folders, and tear them down cleanly.

### Commands

- **`git wt clone <url>`**: Clones a repository as a bare repo and sets up an initial `main` worktree.
- **`git wt add <branch-name> [custom-folder-name]`**: Creates a new worktree (or checks out an existing branch) using a short folder name derived from the branch.
- **`git wt clean`**: Interactively find and remove stale worktrees that have already been merged.
- **`git wt status`**: Show a rich status overview of all active worktrees, including sync status and uncommitted changes.
- **`git wt rm <folder-name-or-branch-name>`**: Safely removes a worktree folder and deletes its associated local branch.
- **`git wt sync`**: Synchronizes configured `copies.paths` across all active worktrees using the main worktree as the central hub. Uses file-level modification times to automatically propagate additions, deletions, and modifications. Prompts for conflict resolution in interactive mode or creates conflict marker files when updates are incompatible.

### Configuration (`.devex.conf`)

You can customize `git wt` behavior by placing a `.devex.conf` file at the root of your repository:

```ini
[repo]
base_branch = main
main_worktree_name = main

[symlinks]
# Files or folders to symlink into every new worktree
paths =

[copies]
# Files or folders to copy into every new worktree (synchronized via git wt sync)
paths = .claude,.kiro,.vscode,.env

[worktree]
# Strategies: ticket-prefix (default), full-branch, or custom
naming_strategy = ticket-prefix
```

### Configuration Copy & Synchronization (`git wt sync`)

Unlike symlinks, which share the exact same underlying file, configuration copies are independent files copied into each worktree (e.g. separate `.vscode/settings.json` or `.env` files). Over time, these copies can drift. 

Running `git wt sync` aligns these files across all active worktrees using a file-level bidirectional synchronization system.

#### How it works:
1. **Manifest-Based Tracking**: The tool maintains a state manifest at `$(git rev-parse --git-common-dir)/info/devex/sync-manifest`. Each entry tracks the file path, modification time (`mtime`), and size at the time of the last sync.
2. **Lock Detection**: To prevent synchronization loops and preserve un-merged changes, `git wt sync` automatically blocks execution if any unresolved conflict files (`*.devex-conflict-*`) are present in the main worktree.
3. **Change Detection**: It scans all active worktrees and compares current states against the manifest to group files into additions, modifications, or deletions since the last sync.
4. **Bidirectional Propagation**:
   - **Silently propagated**: Modifications/additions in the main worktree are sent to feature worktrees. Modifications/additions in a single feature worktree are pulled into the main worktree and fanned out.
   - **Interactive Conflict Resolution**: If a file is modified in both the main worktree and a feature worktree (or modified differently in multiple feature worktrees), the tool prompts the user:
     - **Keep Main version**: Overwrites the feature worktree's version with the main version.
     - **Keep Feature version**: Overwrites the main version and fans it out to all other worktrees.
     - **Resolve Later (Default)**: Leaves the main file untouched and creates a named conflict marker file (e.g., `[filename].devex-conflict-[worktree]`) in the main worktree.

---

## dx nb — Jupyter Notebook Utilities

`dx nb` provides a set of tools specifically designed for AI/ML developers to make working with Jupyter Notebooks in Git repositories less painful.

### Commands

- **`dx nb strip [notebooks...]`**: Clears cell outputs and metadata from `.ipynb` files. This is essential for keeping Git history clean and avoiding massive diffs caused by binary data or volatile metadata.
  - *Requires: `nbstripout` or `nbconvert`*
- **`dx nb kernel`**: Automatically registers the current `.venv` as a Jupyter kernel. It intelligently names the kernel after the current **worktree** or directory name, making it easy to identify the correct environment in JupyterLab/Notebook.
  - *Requires: `ipykernel`*
- **`dx nb list`**: Recursively discovers all notebooks in the project and reports their file sizes, cell counts, and "stripped" status (whether they contain output data).
  - *Recommended: `jq` for full metadata reporting*
- **`dx nb diff <notebook> [git-diff-args]`**: Provides a human-readable diff of notebook cells. If `nbdime` is installed, it uses `nbdiff` for a rich experience; otherwise, it falls back to a standard JSON diff with a helpful warning.
  - *Recommended: `nbdime`*

---

## git-ctx — Developer Context Manager

`git ctx` provides local, untracked todo checklists and scratchpad notes specific to your active branch. This data is stored in the repository's `.git/info/devex/contexts/` directory so it stays private to your computer and never gets committed or pushed.

### Commands

- **`git ctx [show]`**: Display the notes and checklist for the current branch.
- **`git ctx edit`**: Open the current branch's notes in your default terminal editor (`$EDITOR`).
- **`git ctx add <task>`**: Quick shortcut to append a new todo item to the checklist.
- **`git ctx done <index>`**: Mark a checklist item as completed.
- **`git ctx undo <index>`**: Mark a completed checklist item as pending.
- **`git ctx rm <index>`**: Delete a checklist item.
- **`git ctx clean`**: Interactively detect and remove context files for local branches that have already been deleted.
- **`git ctx todo [list|add|done|undo|rm|clear]`**: Detailed subcommand interface for managing checklist items.

---

## dx skills — Prompt Skill Sync Manager

`dx skills` bidirectionally synchronizes custom agent skill folders (containing `skill.md` or `SKILL.md` prompt files) between different AI tool paths on your local machine, using a central Master directory as the source of truth.

It supports dynamic tool registration, backup files prior to overwrites/deletions, global and tool-specific filters (allowlists and denylists), and interactive confirmations.

### Configuration & Setup

You can configure `dx skills` globally by adding a `[skills]` section to your global `~/.devex.conf` configuration file:

```ini
[skills]
# Central directory where all your master skills and backups live
master_dir = <path_to_master_skills_dir>

# Active tools list, comma-separated
tools = <tool_1>, <tool_2>

# Custom skill directories for configured tools (optional, delete or customize)
<tool_1>_dir = ~/<tool_1>/skills
<tool_2>_dir = ~/<tool_2>/skills

# Global Filters (Optional)
# sync_only = scrum-update, code-review   # If set, ONLY sync these skills globally
# ignore = experimental-skill             # Globally ignore this skill folder

# Tool-specific Filters (Optional)
# <tool_1>_sync_only = scrum-update       # Only sync scrum-update to <tool_1>
```

If the file `~/.devex.conf` is missing or contains template placeholders (such as `<path_to_master_skills_dir>` or `<tool_1>`), `dx skills` will print a colorized warning message to `stderr` prompting you to check and customize your paths.

### Commands

- **`dx skills [list]`**: Displays a table of all skills, their descriptions (extracted from prompt frontmatter), update states, and last modification times.
- **`dx skills sync`**: Compares skill folders across all registered tools. Displays a preview of pending bidirectional syncs and executes them with automatic safety backups upon confirmation.
- **`dx skills diff <skill>`**: Shows a recursive folder diff between target versions.
- **`dx skills edit <skill>`**: Opens the master version of the skill in your `$EDITOR` and triggers a sync automatically after saving.
- **`dx skills rm <skill>`**: Backs up and removes the skill directory from the master repo and all active tool paths.

### Backup Strategy

Before executing an overwrite (on `sync`) or delete (on `rm`), `dx skills` copies the target folder's current state to a timestamped backup directory in the master directory under `.backups/YYYY-MM-DD-HHMMSS/`.

---

## Python Auto-Venv

The installer optionally includes Python virtual environment automation that:

- Automatically activates `.venv` when you `cd` into a project directory
- Automatically deactivates when you leave the project
- Provides a quick `venv` command to create new virtual environments using `uv`
- **Proactive Initialization:** When entering a directory containing `pyproject.toml` or `requirements.txt` but no `.venv`, the tool will proactively prompt you to initialize one using `uv`.
- **Stale Worktree Warning:** Automatically warns you if you are working in a worktree whose branch has already been merged into the base branch, suggesting `git wt rm .`.

---

## Optional Git Aliases

Handy shortcuts installed via the `git-aliases/install-git-aliases.sh` module.

| Alias | Expands To | Description |
|---|---|---|
| `git a` | `git add` | Stage files |
| `git s` | `git status` | Show status |
| `git d` | `git diff` | Show diff |
| `git bn` | `git rev-parse --abbrev-ref HEAD` | Current branch name |
| `git fet` | `git fetch origin <branch>` | Fetch current branch |
| `git pul` | `git pull origin <branch>` | Pull current branch |
| `git pus` | `git push origin <branch>` | Push current branch |
| `git stas` | `git stash push -p` | Interactive stash |

---

## Requirements

- Git 2.15+
- Bash 4+
- macOS or Linux
- (Optional) `uv`, `jq`, `nbstripout`, `nbconvert`, `ipykernel`, `nbdime`

## License

MIT
