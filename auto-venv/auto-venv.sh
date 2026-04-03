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
