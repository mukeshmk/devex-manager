#!/usr/bin/env bash

# dx-tools/dx-skills-lib.sh
# Shared library for dx skills commands.

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Parse configuration value from ~/.devex.conf (skills section)
parse_config_val() {
    local key="$1"
    local config_file="$HOME/.devex.conf"
    [ -f "$config_file" ] || return 1
    
    local current_section=""
    while IFS= read -r line || [ -n "$line" ]; do
        # Trim leading/trailing whitespace
        line="${line#"${line%%[![:space:]]*}"}"
        line="${line%"${line##*[![:space:]]}"}"
        
        [[ -z "$line" || "$line" == \#* ]] && continue
        
        if [[ "$line" == \[*\] ]]; then
            current_section="${line#\[}"
            current_section="${current_section%\]}"
            continue
        fi
        
        if [[ "$current_section" == "skills" && "$line" == *=* ]]; then
            local k="${line%%=*}"
            local v="${line#*=}"
            k="${k#"${k%%[![:space:]]*}"}"
            k="${k%"${k##*[![:space:]]}"}"
            v="${v#"${v%%[![:space:]]*}"}"
            v="${v%"${v##*[![:space:]]}"}"
            if [ "$k" = "$key" ]; then
                echo "$v"
                return 0
            fi
        fi
    done < "$config_file"
}

# 0. Warning check for configuration template / missing file
if [ "$DX_WARNING_SHOWN" != "true" ]; then
    export DX_WARNING_SHOWN="true"
    devex_config_file="$HOME/.devex.conf"
    if [ ! -f "$devex_config_file" ]; then
        echo -e "${YELLOW}Warning: Global configuration file ~/.devex.conf not found.${NC}" >&2
        echo -e "${YELLOW}Please configure your master skills path and tool directories in ~/.devex.conf.${NC}\n" >&2
    else
        # Check if the config contains placeholder template values
        if grep -q '<path_to_master_skills_dir>' "$devex_config_file" 2>/dev/null || grep -q '<tool_1>' "$devex_config_file" 2>/dev/null; then
            echo -e "${YELLOW}Warning: ~/.devex.conf has not been customized (currently using template defaults).${NC}" >&2
            echo -e "${YELLOW}Please check and update your master skills path and tool directories in ~/.devex.conf.${NC}\n" >&2
        fi
    fi
fi

# 1. Load Master Directory
if [ -n "$DX_MASTER_SKILLS_DIR" ]; then
    MASTER_DIR="$DX_MASTER_SKILLS_DIR"
else
    MASTER_DIR=$(parse_config_val "master_dir" || true)
    if [ -z "$MASTER_DIR" ]; then
        MASTER_DIR="$HOME/personal/skills"
    fi
fi
MASTER_DIR="${MASTER_DIR/#\~/$HOME}"

# 2. Get active tools list
if [ -n "$DX_TOOLS" ]; then
    tools_list="$DX_TOOLS"
else
    tools_list=$(parse_config_val "tools" || true)
    if [ -z "$tools_list" ]; then
        tools_list="claude,kiro,gemini"
    fi
fi
tools_list=$(echo "$tools_list" | tr ',' ' ' | xargs)

# 3. Resolve Tool Paths and Filters
ACTIVE_TOOLS=()
ACTIVE_DIRS=()
ACTIVE_SYNC_RULES=()

for tool in $tools_list; do
    tool_upper=$(echo "$tool" | tr '[:lower:]' '[:upper:]')
    
    env_dir_var="DX_${tool_upper}_SKILLS_DIR"
    dir="${!env_dir_var}"
    if [ -z "$dir" ]; then
        # Fallback to DX_<TOOL>_DIR for backward compatibility/flexibility
        fallback_env_var="DX_${tool_upper}_DIR"
        dir="${!fallback_env_var}"
    fi
    if [ -z "$dir" ]; then
        dir=$(parse_config_val "${tool}_dir" || true)
    fi
    if [ -z "$dir" ]; then
        case "$tool" in
            claude) dir="$HOME/.claude/skills" ;;
            kiro)   dir="$HOME/.kiro/skills" ;;
            gemini) dir="$HOME/.gemini/antigravity/skills" ;;
        esac
    fi
    [ -z "$dir" ] && continue
    dir="${dir/#\~/$HOME}"
    
    if [ -d "$dir" ]; then
        ACTIVE_TOOLS+=("$tool")
        ACTIVE_DIRS+=("$dir")
        
        env_sync_var="DX_${tool_upper}_SYNC_ONLY"
        sync_only="${!env_sync_var}"
        if [ -z "$sync_only" ]; then
            sync_only=$(parse_config_val "${tool}_sync_only" || true)
        fi
        ACTIVE_SYNC_RULES+=("$sync_only")
    fi
done

# 4. Load Global Filters
if [ -n "$DX_SYNC_ONLY" ]; then
    GLOBAL_SYNC_ONLY="$DX_SYNC_ONLY"
else
    GLOBAL_SYNC_ONLY=$(parse_config_val "sync_only" || true)
fi

if [ -n "$DX_IGNORE" ]; then
    GLOBAL_IGNORE="$DX_IGNORE"
else
    GLOBAL_IGNORE=$(parse_config_val "ignore" || true)
fi

# Helper functions
in_list() {
    local item="$1"
    local list="$2"
    [ -z "$list" ] && return 1
    [[ ",${list//[[:space:]]/}," == *",${item},"* ]]
}

get_dir_mtime() {
    local dir="$1"
    [ -d "$dir" ] || { echo 0; return; }
    local max_mtime=0
    if [[ "$OSTYPE" == "darwin"* ]]; then
        max_mtime=$(find "$dir" -type f -exec stat -f %m {} + 2>/dev/null | sort -rn | head -n 1)
        if [ -z "$max_mtime" ]; then
            max_mtime=$(stat -f %m "$dir" 2>/dev/null || echo 0)
        fi
    else
        max_mtime=$(find "$dir" -type f -exec stat -c %Y {} + 2>/dev/null | sort -rn | head -n 1)
        if [ -z "$max_mtime" ]; then
            max_mtime=$(stat -c %Y "$dir" 2>/dev/null || echo 0)
        fi
    fi
    echo "$max_mtime"
}

parse_frontmatter_field() {
    local file="$1"
    local field="$2"
    [ -f "$file" ] || return 1
    awk -v field="$field" '
        BEGIN { count=0 }
        /^---$/ { count++; next }
        count == 1 && $0 ~ "^" field ":" {
            sub("^" field "[[:space:]]*:[[:space:]]*", "")
            gsub(/^["\x27]|["\x27]$/, "")
            print
            exit
        }
        count > 1 { exit }
    ' "$file"
}

get_skill_file() {
    local dir="$1"
    if [ -f "$dir/SKILL.md" ]; then
        echo "$dir/SKILL.md"
    elif [ -f "$dir/skill.md" ]; then
        echo "$dir/skill.md"
    fi
}

get_all_skills() {
    local dirs_to_scan=()
    [ -d "$MASTER_DIR" ] && dirs_to_scan+=("$MASTER_DIR")
    for d in "${ACTIVE_DIRS[@]}"; do
        dirs_to_scan+=("$d")
    done
    
    local all_skills=""
    for d in "${dirs_to_scan[@]}"; do
        [ -d "$d" ] || continue
        for s in "$d"/*; do
            [ -d "$s" ] || continue
            local name=$(basename "$s")
            [[ "$name" == ".backups" || "$name" == "backups" ]] && continue
            if [ -f "$s/SKILL.md" ] || [ -f "$s/skill.md" ]; then
                all_skills="$all_skills $name"
            fi
        done
    done
    echo "$all_skills" | tr ' ' '\n' | sort -u | xargs
}

format_date() {
    local epoch="$1"
    [ "$epoch" -eq 0 ] && { echo "Never"; return; }
    if [[ "$OSTYPE" == "darwin"* ]]; then
        date -r "$epoch" "+%Y-%m-%d %H:%M:%S" 2>/dev/null || echo "$epoch"
    else
        date -d "@$epoch" "+%Y-%m-%d %H:%M:%S" 2>/dev/null || echo "$epoch"
    fi
}

is_ignored() {
    local name="$1"
    if [ -n "$GLOBAL_SYNC_ONLY" ]; then
        if ! in_list "$name" "$GLOBAL_SYNC_ONLY"; then
            return 0 # true (ignored)
        fi
    fi
    if [ -n "$GLOBAL_IGNORE" ]; then
        if in_list "$name" "$GLOBAL_IGNORE"; then
            return 0 # true (ignored)
        fi
    fi
    return 1 # false (not ignored)
}
