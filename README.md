# DevEx Manager — Developer Experience Tools

A collection of lightweight CLI tools to enhance your development workflow. DevEx Manager includes `git wt` for managing Git worktrees, `git nb` for Jupyter Notebook utilities, and Python auto-venv tools for seamless virtual environment management.

## Key Features

- **`git wt` (Worktree Manager):** Simplify working with bare repositories and multiple worktrees.
- **`git nb` (Notebook Utilities):** Essential tools for AI/ML developers to manage Jupyter Notebooks and kernels.
- **Python Auto-Venv:** Transparent activation/deactivation of `.venv` and proactive initialization using `uv`.
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
1. Downloads/Copies `git-wt`, `git-nb`, and the sub-command scripts to `~/.local/bin/`
2. Makes all scripts executable
3. **Optional:** Prompts to install a set of handy [Git aliases](#optional-git-aliases)
4. **Optional:** Prompts to install [Python auto-venv tools](#python-auto-venv)
5. Sets up shell auto-completion for `git wt` and `git nb` commands
6. Adds `~/.local/bin` to your `PATH` in `~/.zshrc` or `~/.bashrc`

After installing, open a new terminal or run `source ~/.zshrc` (or your respective shell config).

## git-wt — Git Worktree Manager

A lightweight CLI tool that wraps `git worktree` to simplify working with bare repositories and multiple worktrees. Instead of juggling branches in a single checkout, `git wt` lets you clone repos as bare, spin up worktrees as folders, and tear them down cleanly — all with short, memorable commands.

### `git wt clone <url>`
Clones a repository as a bare repo and sets up an initial `main` worktree.

### `git wt add <branch-name> [custom-folder-name]`
Creates a new worktree (or checks out an existing branch) using a short folder name derived from the branch.

### `git wt clean`
Interactively find and remove stale worktrees that have already been merged.

### `git wt status`
Show a rich status overview of all active worktrees, including sync status and uncommitted changes.

### `git wt rm <folder-name-or-branch-name>`
Safely removes a worktree folder and deletes its associated local branch.

---

## git-nb — Jupyter Notebook Utilities

`git nb` provides a set of tools specifically designed for AI/ML developers to make working with Jupyter Notebooks in Git repositories less painful.

### `git nb strip [notebooks...]`
Clears cell outputs and metadata from `.ipynb` files. This is essential for keeping Git history clean and avoiding massive diffs caused by binary data or volatile metadata.
*Requires: `nbconvert` (pip install nbconvert)*

```bash
# Strip all notebooks in current directory
git nb strip *.ipynb
```

### `git nb kernel`
Automatically registers the current `.venv` as a Jupyter kernel. It intelligently names the kernel after the current **worktree** or directory name, making it easy to identify the correct environment in JupyterLab/Notebook.
*Requires: `ipykernel` (pip install ipykernel)*

```bash
# Run inside your worktree/project with an active .venv
git nb kernel
```

### `git nb list`
Recursively discovers all notebooks in the project and reports their file sizes and "stripped" status (whether they contain output data).

```bash
git nb list
```

### `git nb diff <notebook> [git-diff-args]`
Provides a human-readable diff of notebook cells. If `nbdime` is installed, it uses `nbdiff` for a rich experience; otherwise, it falls back to a standard JSON diff with a helpful warning.
*Recommended: `nbdime` (pip install nbdime)*

---

## Python Auto-Venv

The installer optionally includes Python virtual environment automation that:

- Automatically activates `.venv` when you `cd` into a project directory
- Automatically deactivates when you leave the project
- Provides a quick `venv` command to create new virtual environments using `uv`
- **Proactive Initialization:** When entering a directory containing `pyproject.toml` or `requirements.txt` but no `.venv`, the tool will proactively prompt you to initialize one using `uv`.
- **Stale Worktree Warning:** Automatically warns you if you are working in a worktree whose branch has already been merged into the base branch.

### Usage

```bash
# Navigate into a project with a .venv
cd my-python-project
# → .venv automatically activated

# Create a new venv quickly (requires uv)
cd new-project
venv
# → Creates .venv using uv, syncs dependencies, and activates it
```

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
- (Optional) `uv`, `nbconvert`, `ipykernel`, `nbdime`

## License

MIT
