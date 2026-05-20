#!/usr/bin/env bash

# git-ctx-helper.sh: Shared logic for git-ctx sub-commands
set -e

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BOLD='\033[1m'
NC='\033[0m'

# Ensure we are in a git repository
if ! git rev-parse --is-inside-work-tree &>/dev/null; then
    echo -e "${RED}Error: Not a git repository (or any of the parent directories)${NC}"
    exit 1
fi

# Locate the common git directory (supports git worktrees)
git_common_dir="$(cd "$(git rev-parse --git-common-dir 2>/dev/null)" && pwd)"
ctx_dir="$git_common_dir/info/devex/contexts"
mkdir -p "$ctx_dir"

# Get current branch and sanitize it for a safe filename
branch_name="$(git rev-parse --abbrev-ref HEAD 2>/dev/null)"
if [ -z "$branch_name" ] || [ "$branch_name" = "HEAD" ]; then
    echo -e "${RED}Error: Not currently on a branch (detached HEAD)${NC}"
    exit 1
fi

# Replace slashes with double underscores to keep a flat file directory
safe_branch_name="${branch_name//\//__}"
ctx_file="$ctx_dir/${safe_branch_name}.md"

# Initialize context file if it doesn't exist
if [ ! -f "$ctx_file" ]; then
    cat <<EOF > "$ctx_file"
# Context: $branch_name

## Todos

## Notes
EOF
fi

# Helper: Parse all todo items from the file.
# Outputs: index|line_number|status|original_line
get_todos() {
    local in_todos=false
    local index=0
    local line_num=0
    
    # Store regex patterns in variables to avoid syntax errors in conditional expressions
    local heading_pattern='^##?[[:space:]]+'
    local todos_heading_pattern='^##[[:space:]]+[Tt][Oo][Dd][Oo][Ss]'
    local todo_pattern='^[[:space:]]*- \[[ xX]\][[:space:]]+(.*)'
    local done_pattern='^[[:space:]]*- \[[xX]\]'
    
    while IFS= read -r line || [[ -n "$line" ]]; do
        ((line_num++))
        
        # Heading match
        if [[ "$line" =~ $heading_pattern ]]; then
            if [[ "$line" =~ $todos_heading_pattern ]]; then
                in_todos=true
                continue
            else
                in_todos=false
            fi
        fi
        
        if [ "$in_todos" = true ]; then
            # Match "- [ ] text" or "- [x] text"
            if [[ "$line" =~ $todo_pattern ]]; then
                ((index++))
                local status="pending"
                if [[ "$line" =~ $done_pattern ]]; then
                    status="done"
                fi
                echo "$index|$line_num|$status|$line"
            fi
        fi
    done < "$ctx_file"
}

# Helper: Get original file line number for a given todo index
get_line_num_for_index() {
    local target_idx="$1"
    local found=""
    while IFS='|' read -r idx line_num status line; do
        if [ "$idx" -eq "$target_idx" ] 2>/dev/null; then
            found="$line_num"
            break
        fi
    done < <(get_todos)
    echo "$found"
}
