#!/usr/bin/env bash

# --- Auto activate/deactivate venv ---
export _LAST_VENV_PATH=""
export _AUTO_VENV_PROMPTED_DIR=""

_auto_venv_find() {
    local dir="$PWD"
    while [ "$dir" != "/" ]; do
        if [ -e "$dir/.venv/bin/activate" ]; then
            echo "$dir/.venv"
            return
        fi
        dir="$(dirname "$dir")"
    done
    echo ""
}

_auto_venv_check_init() {
    # Only prompt if no venv is active
    if [ -z "$VIRTUAL_ENV" ]; then
        # Find project root (where manifest exists)
        local dir="$PWD"
        local project_root=""
        while [ "$dir" != "/" ]; do
            if [ -f "$dir/pyproject.toml" ] || [ -f "$dir/requirements.txt" ]; then
                project_root="$dir"
                break
            fi
            dir="$(dirname "$dir")"
        done

        # If project root found and we haven't prompted for THIS project root in this session
        if [ -n "$project_root" ] && [ "$_AUTO_VENV_PROMPTED_DIR" != "$project_root" ]; then
            if command -v uv &> /dev/null; then
                echo -e "\033[0;34mPython project detected at $project_root. Would you like to initialize a .venv using uv? (y/n)\033[0m"
                read -p "> " -n 1 -r
                echo ""
                if [[ $REPLY =~ ^[Yy]$ ]]; then
                    # Navigate to project root to run venv
                    (cd "$project_root" && venv)
                fi
                export _AUTO_VENV_PROMPTED_DIR="$project_root"
            fi
        fi
    fi
}

_auto_stale_wt_check() {
    # 1. Quick check for git
    if ! command -v git &> /dev/null; then return; fi

    # 2. Check if we are in a git worktree
    local is_worktree=$(git rev-parse --is-inside-work-tree 2>/dev/null)
    if [ "$is_worktree" != "true" ]; then return; fi

    # 3. Check if we are in a worktree branch (and not main/bare)
    local branch=$(git rev-parse --abbrev-ref HEAD 2>/dev/null)
    if [ -z "$branch" ] || [ "$branch" == "HEAD" ]; then return; fi

    # 4. Resolve repo root to load config
    local git_common_dir=$(git rev-parse --git-common-dir 2>/dev/null)
    if [ -z "$git_common_dir" ]; then return; fi
    local repo_root="$(cd "$git_common_dir/.." && pwd)"

    # 5. Load config to get base branch
    # Try to find devex-lib.sh relative to this script's directory (if installed)
    # or just use a default 'main' if it fails
    local base_branch="main"
    local script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd 2>/dev/null)"
    if [ -f "$script_dir/devex-lib.sh" ]; then
        # We need to be careful not to pollute current environment too much, 
        # but devex_load_config is designed for this.
        source "$script_dir/devex-lib.sh"
        devex_load_config "$repo_root"
        base_branch="$DEVEX_BASE_BRANCH"
    fi

    # Skip if we are already on the base branch
    if [ "$branch" == "$base_branch" ]; then return; fi

    # 6. Check if branch is merged into base branch
    # Note: Use local check only for performance
    if git branch --merged "$base_branch" | grep -qx "[ *]*$branch"; then
        echo -e "\033[1;33m[DevEx] Branch '$branch' has been merged into '$base_branch'.\033[0m"
        echo -e "\033[0;34m        Run 'git wt rm .' to clean up this worktree.\033[0m"
    fi
}

_auto_venv_switch() {
    local new_venv="$(_auto_venv_find)"
    if [ -n "$VIRTUAL_ENV" ] && [ "$VIRTUAL_ENV" != "$new_venv" ]; then
        # Deactivate old venv if switching or leaving project
        # In some shells 'deactivate' might not be defined if no venv was active
        if command -v deactivate &> /dev/null; then
            deactivate
        fi
        export _LAST_VENV_PATH=""
    fi
    if [ -n "$new_venv" ] && [ "$VIRTUAL_ENV" != "$new_venv" ]; then
        # Activate new venv if not already activated
        source "$new_venv/bin/activate"
        export _LAST_VENV_PATH="$new_venv"
    fi

    # After switching, check if we should prompt for init
    _auto_venv_check_init

    # Check for stale worktree
    _auto_stale_wt_check
}

# Override cd to automatically switch venvs
cd() {
    builtin cd "$@" && _auto_venv_switch
}

# Quick venv creation using uv
venv() {
    if command -v uv &> /dev/null; then
        uv venv .venv
        source ".venv/bin/activate"
        if [ -f "pyproject.toml" ]; then
            uv sync
        fi
        echo -e "\033[0;32m✓ Virtual environment created, activated, and synced using uv.\033[0m"
    else
        echo -e "\033[0;31mError: 'uv' is not installed. Please install it first.\033[0m"
    fi
}

# Run the switch once when the terminal first opens
_auto_venv_switch
