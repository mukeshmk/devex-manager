#!/usr/bin/env bash

set -e # Exit on error

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

INSTALL_DIR="$HOME/.local/bin"
WT_TOOLS_DIR="$INSTALL_DIR/git-wt-tools"
CTX_TOOLS_DIR="$INSTALL_DIR/git-ctx-tools"
DX_SKILLS_TOOLS_DIR="$INSTALL_DIR/dx-skills-tools"
DX_NB_TOOLS_DIR="$INSTALL_DIR/dx-nb-tools"
REPO_RAW_URL="https://raw.githubusercontent.com/mukeshmk/devex-manager/main"

# Detect if we are running from a local clone or remotely via curl
# When piped to bash, BASH_SOURCE[0] is often empty or '-'
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd 2>/dev/null || pwd)"
IS_REMOTE=false

if [[ ! -f "$SCRIPT_DIR/git-wt" ]]; then
    IS_REMOTE=true
    # Check for curl if remote
    if ! command -v curl &> /dev/null; then
        echo -e "${RED}Error: 'curl' is required for remote installation.${NC}"
        exit 1
    fi
fi

# Helper to fetch files (local or remote)
fetch_file() {
    local src_path="$1"
    local dest_path="$2"

    if [ "$IS_REMOTE" = true ]; then
        curl -fsSL "$REPO_RAW_URL/$src_path" -o "$dest_path"
    else
        cp "$SCRIPT_DIR/$src_path" "$dest_path"
    fi
}

echo -e "${BLUE}Installing DevEx Manager...${NC}"
if [ "$IS_REMOTE" = true ]; then
    echo -e "${BLUE}Mode: Remote installation from GitHub${NC}"
else
    echo -e "${BLUE}Mode: Local installation${NC}"
fi

# 1. Create the destination directories
mkdir -p "$INSTALL_DIR"
mkdir -p "$WT_TOOLS_DIR"
mkdir -p "$DX_SKILLS_TOOLS_DIR"
mkdir -p "$DX_NB_TOOLS_DIR"

# 2. Copy executable files
echo "Installing routers and tools to $INSTALL_DIR..."
fetch_file "git-wt" "$INSTALL_DIR/git-wt"
fetch_file "dx" "$INSTALL_DIR/dx"


echo "Installing worktree sub-commands to $WT_TOOLS_DIR..."
fetch_file "git-wt-tools/git-wt-clone" "$WT_TOOLS_DIR/git-wt-clone"
fetch_file "git-wt-tools/git-wt-add" "$WT_TOOLS_DIR/git-wt-add"
fetch_file "git-wt-tools/git-wt-rm" "$WT_TOOLS_DIR/git-wt-rm"
fetch_file "git-wt-tools/git-wt-clean" "$WT_TOOLS_DIR/git-wt-clean"
fetch_file "git-wt-tools/git-wt-status" "$WT_TOOLS_DIR/git-wt-status"
fetch_file "git-wt-tools/devex-lib.sh" "$WT_TOOLS_DIR/devex-lib.sh"

echo "Installing dx skills tools to $DX_SKILLS_TOOLS_DIR..."
fetch_file "dx-skills-tools/dx-skills" "$DX_SKILLS_TOOLS_DIR/dx-skills"
fetch_file "dx-skills-tools/dx-skills-lib.sh" "$DX_SKILLS_TOOLS_DIR/dx-skills-lib.sh"
fetch_file "dx-skills-tools/dx-skills-list" "$DX_SKILLS_TOOLS_DIR/dx-skills-list"
fetch_file "dx-skills-tools/dx-skills-sync" "$DX_SKILLS_TOOLS_DIR/dx-skills-sync"
fetch_file "dx-skills-tools/dx-skills-diff" "$DX_SKILLS_TOOLS_DIR/dx-skills-diff"
fetch_file "dx-skills-tools/dx-skills-edit" "$DX_SKILLS_TOOLS_DIR/dx-skills-edit"
fetch_file "dx-skills-tools/dx-skills-rm" "$DX_SKILLS_TOOLS_DIR/dx-skills-rm"

echo "Installing dx notebook tools to $DX_NB_TOOLS_DIR..."
fetch_file "dx-nb-tools/dx-nb" "$DX_NB_TOOLS_DIR/dx-nb"
fetch_file "dx-nb-tools/dx-nb-strip" "$DX_NB_TOOLS_DIR/dx-nb-strip"
fetch_file "dx-nb-tools/dx-nb-diff" "$DX_NB_TOOLS_DIR/dx-nb-diff"
fetch_file "dx-nb-tools/dx-nb-kernel" "$DX_NB_TOOLS_DIR/dx-nb-kernel"
fetch_file "dx-nb-tools/dx-nb-list" "$DX_NB_TOOLS_DIR/dx-nb-list"

# Ask if user wants to install git-ctx
INSTALL_GIT_CTX=false
echo -e "\n${YELLOW}Would you like to install the git-ctx (Developer Context Manager) tools?${NC}"
read -p "(y/n) " -n 1 -r
echo ""
if [[ $REPLY =~ ^[Yy]$ ]]; then
    INSTALL_GIT_CTX=true
fi

if [ "$INSTALL_GIT_CTX" = true ]; then
    mkdir -p "$CTX_TOOLS_DIR"
    fetch_file "git-ctx" "$INSTALL_DIR/git-ctx"

    echo "Installing context sub-commands to $CTX_TOOLS_DIR..."
    fetch_file "git-ctx-tools/git-ctx-helper.sh" "$CTX_TOOLS_DIR/git-ctx-helper.sh"
    fetch_file "git-ctx-tools/git-ctx-show" "$CTX_TOOLS_DIR/git-ctx-show"
    fetch_file "git-ctx-tools/git-ctx-edit" "$CTX_TOOLS_DIR/git-ctx-edit"
    fetch_file "git-ctx-tools/git-ctx-add" "$CTX_TOOLS_DIR/git-ctx-add"
    fetch_file "git-ctx-tools/git-ctx-done" "$CTX_TOOLS_DIR/git-ctx-done"
    fetch_file "git-ctx-tools/git-ctx-undo" "$CTX_TOOLS_DIR/git-ctx-undo"
    fetch_file "git-ctx-tools/git-ctx-rm" "$CTX_TOOLS_DIR/git-ctx-rm"
    fetch_file "git-ctx-tools/git-ctx-clear" "$CTX_TOOLS_DIR/git-ctx-clear"
    fetch_file "git-ctx-tools/git-ctx-clean" "$CTX_TOOLS_DIR/git-ctx-clean"
    fetch_file "git-ctx-tools/git-ctx-todo" "$CTX_TOOLS_DIR/git-ctx-todo"
fi

echo "Installing completion scripts to $WT_TOOLS_DIR..."
fetch_file "git-wt-completion.sh" "$WT_TOOLS_DIR/git-wt-completion.sh"
fetch_file "dx-completion.sh" "$WT_TOOLS_DIR/dx-completion.sh"
if [ "$INSTALL_GIT_CTX" = true ]; then
    fetch_file "git-ctx-completion.sh" "$WT_TOOLS_DIR/git-ctx-completion.sh"
fi

# 3. Make the scripts executable
chmod +x "$INSTALL_DIR/git-wt"
chmod +x "$INSTALL_DIR/dx"
chmod +x "$WT_TOOLS_DIR"/git-wt-*
chmod +x "$WT_TOOLS_DIR/devex-lib.sh"
chmod +x "$DX_SKILLS_TOOLS_DIR"/dx-*
chmod +x "$DX_NB_TOOLS_DIR"/dx-*

if [ "$INSTALL_GIT_CTX" = true ]; then
    chmod +x "$INSTALL_DIR/git-ctx"
    chmod +x "$CTX_TOOLS_DIR"/git-ctx-*
    chmod +x "$CTX_TOOLS_DIR/git-ctx-helper.sh"
fi

# 4. Optional: Install Git Aliases
echo -e "\n${YELLOW}Would you like to install the recommended Git shortcuts (e.g., 'git s' for status)?${NC}"
read -p "(y/n) " -n 1 -r
echo ""
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "Installing Git aliases..."
    if [ "$IS_REMOTE" = true ]; then
        curl -fsSL "$REPO_RAW_URL/git-aliases/install-git-aliases.sh" | bash
    else
        bash "$SCRIPT_DIR/git-aliases/install-git-aliases.sh"
    fi
else
    echo "Skipping Git aliases."
fi

# 5. Optional: Install Python Auto-Venv Tools
echo -e "\n${YELLOW}Would you like to install the Python auto-venv tools (auto-activates .venv when you cd into a directory)?${NC}"
read -p "(y/n) " -n 1 -r
echo ""
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "Installing auto-venv script..."
    fetch_file "auto-venv/auto-venv.sh" "$WT_TOOLS_DIR/auto-venv.sh"
    INSTALL_AUTO_VENV=true
    echo -e "${GREEN}✓ Auto-venv tools staged for installation!${NC}"
else
    INSTALL_AUTO_VENV=false
    echo "Skipping Python auto-venv tools."
fi

# 6. Detect shell and set up PATH & Auto-completion
SHELL_RC=""
if [[ "$SHELL" == */zsh ]]; then
    SHELL_RC="$HOME/.zshrc"
elif [[ "$SHELL" == */bash ]]; then
    if [ -f "$HOME/.bash_profile" ]; then
        SHELL_RC="$HOME/.bash_profile"
    else
        SHELL_RC="$HOME/.bashrc"
    fi
fi

if [ -n "$SHELL_RC" ]; then
    # Create the DevEx Manager block if it doesn't exist
    if ! grep -q '# >>> DevEx Manager >>>' "$SHELL_RC"; then
        echo -e "\n${BLUE}Initializing DevEx Manager block in $(basename "$SHELL_RC")...${NC}"
        echo "" >> "$SHELL_RC"
        echo '# >>> DevEx Manager >>>' >> "$SHELL_RC"
        echo '# <<< DevEx Manager <<<' >> "$SHELL_RC"
    fi

    # Function to add a line inside the block if it's missing
    add_to_devex_block() {
        local line="$1"
        if ! grep -Fq "$line" "$SHELL_RC"; then
            # Insert before the closing marker using awk (portable on macOS/Darwin)
            awk -v line="$line" '/# <<< DevEx Manager <<</ { print line } { print }' "$SHELL_RC" > "${SHELL_RC}.tmp" && \
            mv "${SHELL_RC}.tmp" "$SHELL_RC"
            return 0
        fi
        return 0
    }

    # Setup PATH
    if [[ ":$PATH:" != *":$HOME/.local/bin:"* ]] && ! grep -q '.local/bin' "$SHELL_RC"; then
        echo -e "\n${YELLOW}Adding $INSTALL_DIR to your PATH in $(basename "$SHELL_RC")...${NC}"
        add_to_devex_block 'export PATH="$HOME/.local/bin:$PATH"'
    else
        echo -e "\nPATH already configured in $(basename "$SHELL_RC")."
    fi

    # Setup Auto-completion
    if ! grep -q "git-wt-completion.sh" "$SHELL_RC"; then
        echo -e "${YELLOW}Adding auto-completion to $(basename "$SHELL_RC")...${NC}"
        
        # Zsh needs bashcompinit to run bash completion scripts properly
        if [[ "$SHELL" == */zsh ]]; then
            add_to_devex_block 'autoload -Uz compinit && compinit'
            add_to_devex_block 'autoload -Uz bashcompinit && bashcompinit'
        fi
        
        add_to_devex_block "source \"$WT_TOOLS_DIR/git-wt-completion.sh\""
        add_to_devex_block "source \"$WT_TOOLS_DIR/dx-completion.sh\""
        if [ "$INSTALL_GIT_CTX" = true ]; then
            add_to_devex_block "source \"$WT_TOOLS_DIR/git-ctx-completion.sh\""
        fi
    else
        # Make sure git-ctx-completion and dx-completion are loaded if others were already configured
        add_to_devex_block "source \"$WT_TOOLS_DIR/dx-completion.sh\""
        if [ "$INSTALL_GIT_CTX" = true ]; then
            add_to_devex_block "source \"$WT_TOOLS_DIR/git-ctx-completion.sh\""
        fi
        echo -e "Auto-completion already configured in $(basename "$SHELL_RC")."
    fi

    # Setup Auto-Venv (if the user said yes)
    if [ "$INSTALL_AUTO_VENV" = true ]; then
        if ! grep -q "auto-venv.sh" "$SHELL_RC"; then
            echo -e "${YELLOW}Adding auto-venv to $(basename "$SHELL_RC")...${NC}"
            add_to_devex_block "source \"$WT_TOOLS_DIR/auto-venv.sh\""
        else
            echo -e "Auto-venv already configured in $(basename "$SHELL_RC")."
        fi
    fi
else
    echo -e "\n${YELLOW}Warning: Could not detect bash or zsh configuration file.${NC}"
    echo -e "Please ensure '$INSTALL_DIR' is in your PATH manually."
fi

# 6. Initialize global ~/.devex.conf file with template values if needed
DEVEX_CONF="$HOME/.devex.conf"
if [ ! -f "$DEVEX_CONF" ]; then
    echo -e "\n${BLUE}Creating template config file at $DEVEX_CONF...${NC}"
    cat <<EOF > "$DEVEX_CONF"
[skills]
# Central directory where all your master skills and backups live
master_dir = <path_to_master_skills_dir>

# Active tools list, comma-separated
tools = <tool_1>, <tool_2>

# Custom skill directories for configured tools (optional, delete or customize)
<tool_1>_dir = ~/<tool_1>/skills
<tool_2>_dir = ~/<tool_2>/skills
EOF
elif ! grep -q '\[skills\]' "$DEVEX_CONF"; then
    echo -e "\n${BLUE}Adding [skills] template to existing $DEVEX_CONF...${NC}"
    cat <<EOF >> "$DEVEX_CONF"

[skills]
# Central directory where all your master skills and backups live
master_dir = <path_to_master_skills_dir>

# Active tools list, comma-separated
tools = <tool_1>, <tool_2>

# Custom skill directories for configured tools (optional, delete or customize)
<tool_1>_dir = ~/<tool_1>/skills
<tool_2>_dir = ~/<tool_2>/skills
EOF
fi

echo -e "\n${GREEN}✓ Installation complete!${NC}"
echo -e "To start using your new commands, open a new terminal tab, or run:"
echo -e "  ${BLUE}source $SHELL_RC${NC}"
