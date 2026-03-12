#!/usr/bin/env bash

# --- Auto activate/deactivate venv ---
export _LAST_VENV_PATH=""

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

_auto_venv_switch() {
    local new_venv="$(_auto_venv_find)"
    if [ -n "$VIRTUAL_ENV" ] && [ "$VIRTUAL_ENV" != "$new_venv" ]; then
        # Deactivate old venv if switching or leaving project
        deactivate
        export _LAST_VENV_PATH=""
    fi
    if [ -n "$new_venv" ] && [ "$VIRTUAL_ENV" != "$new_venv" ]; then
        # Activate new venv if not already activated
        source "$new_venv/bin/activate"
        export _LAST_VENV_PATH="$new_venv"
    fi
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
        echo -e "\033[0;32m✓ Virtual environment created and activated using uv.\033[0m"
    else
        echo -e "\033[0;31mError: 'uv' is not installed. Please install it first.\033[0m"
    fi
}

# Run the switch once when the terminal first opens
_auto_venv_switch
