# DevEx Manager — Developer Experience Tools

A single-binary collection of CLI tools to enhance your development workflow. DevEx Manager includes `git wt` for managing Git worktrees, `dx nb` for Jupyter Notebook utilities, `dx ctx` for branch-specific developer scratchpads, and Python auto-venv tools for seamless virtual environment management.

## Key Features

- **`git wt` (Worktree Manager):** Simplify working with bare repositories and multiple worktrees.
- **`dx nb` (Notebook Utilities):** Essential tools for AI/ML developers to manage Jupyter Notebooks and kernels.
- **`dx ctx` (Context Manager):** Local, untracked todo checklists and scratchpad notes per branch.
- **`dx skills` (Prompt Skill Sync Manager):** Bidirectionally synchronize prompt skill folders across multiple AI tools (Claude Code, Kiro, Antigravity, etc.) using a central Master folder.
- **Python Auto-Venv:** Transparent activation/deactivation of `.venv`, proactive initialization using `uv`, and stale worktree detection.
- **Git Aliases:** High-productivity shortcuts for common Git operations.

## Installation

DevEx Manager is a single Go binary: `dx`, with `git-wt` as a symlink to it (so `git wt ...` works).

### Remote Installation (Recommended)

```bash
bash -c "$(curl -fsSL https://raw.githubusercontent.com/mukeshmk/devex-manager/main/install.sh)"
```

This downloads the latest GitHub Release binary for your machine (macOS or Linux, amd64 or arm64). You do not need Go.

### Building from a clone

```bash
git clone https://github.com/mukeshmk/devex-manager.git
cd devex-manager
bash install.sh              # builds with `go build` if Go is on PATH, else downloads the release
# or just build:
go build -o dist/dx ./cmd/dx && ln -sf dx dist/git-wt
go test ./...
```

### What the installer does:
1. Installs `dx` and the `git-wt` symlink to `~/.local/bin/`.
2. **Optional:** Prompts to install a set of handy [Git aliases](#optional-git-aliases).
3. **Optional:** Prompts to install [Python auto-venv tools](#python-auto-venv).
4. Generates shell completions (`dx completion zsh|bash`) and adds `~/.local/bin` to your `PATH` in a managed `# >>> DevEx Manager >>>` block in your `~/.zshrc` or `~/.bashrc`.

After installing, open a new terminal or run `source ~/.zshrc` (or your respective shell config).

### Behaviour notes

- `git wt rm` of a target that is not found exits 1.
- Errors go to stderr.
- `git wt clean` skips dirty worktrees by default (see [Dirty-worktree refusal](#dirty-worktree-refusal)).
- `git wt sync` shows a plan first and backs up files before overwriting or deleting them.
- With no terminal, `git wt sync` does not read stdin: it applies the plan with each question's default (conflicts default to "Resolve Later"). Other prompts (`git wt clean`, `dx skills`, `dx ctx`) read stdin, and EOF means "no" (nothing destructive happens). `--yes/-y` accepts a plan.
- `git wt rm <name>` matches an exact worktree path first, then a folder name, then a branch name. A partial branch name is refused with a "Did you mean" list instead of deleting the first match.

---

## git-wt — Git Worktree Manager

A lightweight CLI tool that wraps `git worktree` to simplify working with bare repositories and multiple worktrees. Instead of juggling branches in a single checkout, `git wt` lets you clone repos as bare, spin up worktrees as folders, and tear them down cleanly.

### Commands

- **`git wt`** (no args) / **`git wt status`**: On a terminal, opens the full-screen worktree app (see below). Piped, prints a plain status table.
- **`git wt clone <url>`**: Clones a repository as a bare repo and sets up an initial `main` worktree.
- **`git wt add <branch-name> [custom-folder-name]`**: Creates a new worktree (or checks out an existing branch) using a short folder name derived from the branch.
- **`git wt clean`**: Interactively find and remove stale worktrees (merged, or whose remote branch is gone). Shows what would be lost before confirming; `--yes`/`-y` takes the preselection. **Note:** This command checks if configuration copies are synchronized across all worktrees before allowing any deletions, and aborts if they are out of sync.
- **`git wt status`**: Show a rich status overview of all active worktrees, including sync status and uncommitted changes.
- **`git wt rm <folder-name-or-branch-name>`**: Safely removes a worktree folder and deletes its associated local branch. Refuses dirty worktrees unless `--force`/`-f` (which also skips the sync check). **Note:** This command checks if configuration copies are synchronized across all worktrees before allowing deletion, and aborts if they are out of sync.
- **`git wt sync`**: Synchronizes configured `copies.paths` across all active worktrees using the main worktree as the central hub. Uses file-level modification times to automatically propagate additions, deletions, and modifications. Prompts for conflict resolution in interactive mode or creates conflict marker files when updates are incompatible. Shows the full plan first and asks to confirm. Flags: `--check`/`-c` (report sync status, exit 1 if out of sync), `--dry-run` (print the plan, change nothing), `--yes`/`-y` (accept the plan without asking).

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
5. **Sync Checking (Dry-Run)**: Running `git wt sync --check` (or `-c`) performs a check of all configuration files against the manifest and other worktrees. It identifies and lists any out-of-sync files, exiting with status `1` (or `0` if all files are in sync) without performing any writes.

#### Worktree Deletion Guard

To prevent accidental loss of configuration modifications made inside worktrees, all commands that delete or clean worktrees (`git wt clean` and `git wt rm`) automatically check if configuration copies are synchronized across all worktrees. If they are not in sync, the command will print details about the out-of-sync files, suggest running `git wt sync`, and abort the deletion.

#### Sync backups

Before sync overwrites or deletes any file, the old version is copied to `$(git rev-parse --git-common-dir)/info/devex/sync-backups/YYYY-MM-DD-HHMMSS/<worktree>/<rel-path>`.

#### Dirty-worktree refusal

A worktree is dirty if it has uncommitted changes (untracked files at or under a configured copy path, and configured symlinks that are real symlinks, do not count), or commits that are on no remote, unless the branch is merged into the base branch or its upstream branch was deleted. `git wt clean` leaves dirty worktrees unselected (you must tick them explicitly) and `git wt rm` refuses them without `--force` (which also skips the sync check). When the upstream is gone but local commits remain, the worktree is still preselected, and `clean` and the app show how many local commits are not on any remote. The current worktree, the main worktree and the base branch are never deleted.

### Full-screen apps

On a terminal, two commands open a full-screen app. Piped or scripted, they print plain tables instead.

**`git wt` / `git wt status`**: a lazygit-style panel layout. Four panels stack on the left (1 Status, 2 Worktrees, 3 Context, 4 Sync); a main view (details of the selected panel item) sits top right, and the command log (5) below it.

```
╭─ [1] Status ───────────╮╭─ DEVX-1 ───────────────────────────────────────────╮
│ repo · base main       ││ Branch   feature/DEVX-1-long-branch-name           │
│ sync: no copy paths    ││ Path     ~/code/repo/DEVX-1                        │
╰────────────────────────╯│ Remote   no upstream                               │
╭─ [2] Worktrees ◆ ──────╮│ Merged   yes                                       │
│▸ DEVX-1 clean   no ups…││                                                    │
│  DEVX-2 clean   no ups…││ Changed files                                      │
│  main   clean   no ups…││  none                                              │
│                        ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
╰────────────────────────╯│                                                    │
╭─ [3] Context ──────────╮╰────────────────────────────────────────────────────╯
│ No todos               │╭─ [5] Command log ──────────────────────────────────╮
│                        ││ git worktree list --porcelain                      │
╰────────────────────────╯│                                                    │
╭─ [4] Sync ─────────────╮│                                                    │
│                        ││                                                    │
│                        ││                                                    │
╰────────────────────────╯╰────────────────────────────────────────────────────╯
 / filter · 1-5 panels · ↑↓ move · s sync · c clean · d delete · q quit · ? keys
```

Below 80 columns the four left panels become a 2x2 grid (Status|Sync over Worktrees|Context) above the main view and a 3-row command log. The minimum size is 60x12.

| Key | Action |
|---|---|
| `1`-`5` | Focus a panel |
| `tab`/`right`/`l`, `shift+tab`/`left`/`h` | Next / previous panel |
| `up`/`down` (`k`/`j`) | Move, or scroll the main view or log |
| `enter` | Focus the main view so it scrolls |
| `esc` | Leave the main view, or clear the filter |
| `r` | Refresh |
| `s` | Sync copy paths |
| `c` | Clean stale worktrees (merged, or upstream gone) |
| `d` | Delete the selected worktree |
| `n` | New worktree: pick a remote branch (type to filter) or `+ new branch…` |
| `f` | Fetch all remotes (`git fetch --all --prune`) |
| `?` | Key help |
| `q`/`ctrl+c` | Quit |

Worktrees panel:

| Key | Action |
|---|---|
| `/` | Filter worktrees (`esc` clears) |

Context panel (that branch's `dx ctx` todos):

| Key | Action |
|---|---|
| `a` | Add a todo |
| `space` | Toggle the selected todo |
| `x` | Delete the selected todo |
| `C` | Clear done todos |

Sync, clean and delete run inside the app: questions and confirmations appear as popups over the main view (the panels and command log stay visible, so the plan an action prints is on screen while you answer), and each one calls the same code as `git wt sync`, `git wt clean` and `git wt rm`, with the same safety checks. In-app delete never uses `--force`. While an action runs, `q` and `ctrl+c` do not quit; the status line shows `wait for <action> to finish`. `ctrl+c` in a flow question cancels the whole action (for sync, before anything changes). `esc` depends on the question: a choose question takes its default, a confirm means no, a checklist ticks nothing, and an input cancels.

`n` lists origin branches that have no worktree yet; picking one runs the same code as `git wt add <branch>`, and `+ new branch…` asks for a name and creates it from the base branch (with `naming_strategy = custom`, a second popup asks for the folder name). The new worktree is selected when it is ready. `f` fetches so new remote branches show up in `n`. Git never asks for a password or passphrase inside the app: if fetch needs one, it fails with git's message in the status line.

The command log lists every git command the tool runs plus each action's output. It keeps the last 500 lines and scrolls when focused (`5`).

**`dx skills`** (no subcommand): grid of skills (rows) by master and each tool (columns), with a diff of the selected skill alongside.

| Key | Action |
|---|---|
| `up`/`down` (`k`/`j`) | Move |
| `space` | Tick/untick the pending update |
| `a` | Apply ticked updates |
| `pgup`/`pgdn` | Scroll the diff |
| `/` | Filter (`esc` clears) |
| `q` | Quit |

---

### Using with lazygit

Add custom commands to `~/Library/Application Support/lazygit/config.yml` on macOS (or `~/.config/lazygit/config.yml` on Linux) to open the apps from lazygit. The keys below are examples (free in lazygit 0.66.2's defaults); press `?` in lazygit to check for conflicts with built-in keys in your version.

```yaml
# Return to lazygit straight away when git wt / dx skills quit (no "Press enter to return")
promptToReturnFromSubprocess: false

customCommands:
  - key: 'X'
    context: 'global'
    command: 'git wt'
    output: terminal
    description: 'git wt worktree app'
  - key: 'U'
    context: 'global'
    command: 'dx skills'
    output: terminal
    description: 'dx skills grid'
  - key: 'Y'
    context: 'global'
    command: 'git wt sync'
    output: popup
    description: 'git wt sync'
  # Branches panel, Remotes tab: make a worktree for the selected remote branch
  - key: 'X'
    context: 'remoteBranches'
    command: 'git wt add {{.SelectedRemoteBranch.Name}}'
    output: popup
    description: 'git wt add (worktree for this remote branch)'
  # Branches panel, Local tab: make a worktree for the selected local branch
  - key: 'X'
    context: 'localBranches'
    command: 'git wt add {{.SelectedLocalBranch.Name}}'
    output: popup
    description: 'git wt add (worktree for this branch)'
```

`promptToReturnFromSubprocess: false` skips lazygit's "Press enter to return" after the app quits. Because of it, output from an `output: terminal` command would vanish as soon as the command exits, so sync and add use `output: popup` to keep their output readable. With `output: popup`, `git wt sync` runs without a TTY, so it takes each question's default without asking (Resolve Later for conflicts).

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

## dx-ctx — Developer Context Manager

`dx ctx` provides local, untracked todo checklists and scratchpad notes specific to your active branch. This data is stored in the repository's `<git-common-dir>/info/devex/contexts/` directory so it stays private to your computer and never gets committed or pushed.

### Commands

- **`dx ctx [show]`**: Display the notes and checklist for the current branch.
- **`dx ctx edit`**: Open the current branch's notes in your default terminal editor (`$EDITOR`).
- **`dx ctx add <task>`**: Quick shortcut to append a new todo item to the checklist.
- **`dx ctx done <index>`**: Mark a checklist item as completed.
- **`dx ctx undo <index>`**: Mark a completed checklist item as pending.
- **`dx ctx rm <index>`**: Delete a checklist item.
- **`dx ctx clean`**: Interactively detect and remove context files for local branches that have already been deleted.
- **`dx ctx todo [list|add|done|undo|rm|clear]`**: Detailed subcommand interface for managing checklist items.

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

- **`dx skills`**: Opens the full-screen grid on a terminal; otherwise behaves like `dx skills list`.
- **`dx skills list`**: Always prints a table of all skills, their descriptions (extracted from prompt frontmatter), update states, and last modification times.
- **`dx skills sync`**: Compares skill folders across all registered tools. Displays a preview of pending bidirectional syncs and executes them with automatic safety backups upon confirmation (`--yes`/`-y` accepts).
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

Handy shortcuts installed via the `shell/git-aliases/install.sh` module.

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
- macOS or Linux (amd64 or arm64)
- Go 1.27+ only if building from source
- (Optional) `uv`, `jq`, `nbstripout`, `nbconvert`, `ipykernel`, `nbdime`

## License

MIT
