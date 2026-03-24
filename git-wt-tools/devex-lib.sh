#!/usr/bin/env bash

# git-wt-tools/devex-lib.sh
#
# Shared library for DevEx Manager configuration.
# Usage: source this file, then call devex_load_config <repo_root>
#
# After calling devex_load_config, these variables are set:
#   DEVEX_SYMLINK_PATHS      - comma-separated list of paths (files/dirs) to symlink
#   DEVEX_COPY_PATHS         - comma-separated list of paths (files/dirs) to copy
#   DEVEX_NAMING_STRATEGY    - "ticket-prefix" or "full-branch"
#   DEVEX_MAIN_WORKTREE_NAME - name of the main worktree directory

devex_load_config() {
    local repo_root="$1"
    local config_file="$repo_root/.devex.conf"

    # Defaults
    DEVEX_SYMLINK_PATHS=".claude,.kiro,.vscode"
    DEVEX_COPY_PATHS=""
    DEVEX_NAMING_STRATEGY="ticket-prefix"
    DEVEX_MAIN_WORKTREE_NAME="main"

    # If no config file, keep defaults (backward compat)
    [ -f "$config_file" ] || return 0

    local current_section=""
    while IFS= read -r line || [ -n "$line" ]; do
        # Trim leading/trailing whitespace
        line="${line#"${line%%[![:space:]]*}"}"
        line="${line%"${line##*[![:space:]]}"}"

        # Skip blank lines and comments
        [[ -z "$line" || "$line" == \#* ]] && continue

        # Section header
        if [[ "$line" =~ ^\[([a-zA-Z_]+)\]$ ]]; then
            current_section="${BASH_REMATCH[1]}"
            continue
        fi

        # Key = value
        if [[ "$line" =~ ^([^=]+)=(.*)$ ]]; then
            local key="${BASH_REMATCH[1]}"
            local value="${BASH_REMATCH[2]}"
            # Trim whitespace from key and value
            key="${key#"${key%%[![:space:]]*}"}"
            key="${key%"${key##*[![:space:]]}"}"
            value="${value#"${value%%[![:space:]]*}"}"
            value="${value%"${value##*[![:space:]]}"}"

            case "${current_section}.${key}" in
                symlinks.paths)              DEVEX_SYMLINK_PATHS="$value" ;;
                copies.paths)                DEVEX_COPY_PATHS="$value" ;;
                worktree.naming_strategy)     DEVEX_NAMING_STRATEGY="$value" ;;
                worktree.main_worktree_name)  DEVEX_MAIN_WORKTREE_NAME="$value" ;;
            esac
        fi
    done < "$config_file"
}

# Color constants for error output
_DEVEX_RED='\033[0;31m'
_DEVEX_NC='\033[0m'

devex_resolve_worktree_name() {
    local branch_name="$1"
    local strategy="$2"

    case "$strategy" in
        ticket-prefix)
            echo "$branch_name" | cut -d'-' -f1,2
            ;;
        full-branch)
            echo "$branch_name" | sed 's|/|--|g'
            ;;
        *)
            echo -e "${_DEVEX_RED}Error: Unrecognized naming_strategy '$strategy' in .devex.conf${_DEVEX_NC}" >&2
            return 1
            ;;
    esac
}
