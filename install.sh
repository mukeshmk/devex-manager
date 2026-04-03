#!/usr/bin/env bash

set -e # Exit on error

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

INSTALL_DIR="$HOME/.local/bin"
TOOLS_DIR="$INSTALL_DIR/git-wt-tools"
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

fetch_file() {
    local src_path="$1"
    local dest_path="$2"

    if [ "$IS_REMOTE" = true ]; then
        curl -fsSL "$REPO_RAW_URL/$src_path" -o "$dest_path"
    else
        cp "$SCRIPT_DIR/$src_path" "$dest_path"
    fi
}

echo -e "${BLUE}Installing DevEx Manager (git wt)...${NC}"
if [ "$IS_REMOTE" = true ]; then
    echo -e "${BLUE}Mode: Remote installation from GitHub${NC}"
else
    echo -e "${BLUE}Mode: Local installation${NC}"
fi

# 1. Create the destination directories
mkdir -p "$INSTALL_DIR"
mkdir -p "$TOOLS_DIR"

# 2. Copy executable files
echo "Installing router to $INSTALL_DIR..."
fetch_file "git-wt" "$INSTALL_DIR/git-wt"

echo "Installing sub-commands to $TOOLS_DIR..."
fetch_file "git-wt-tools/git-wt-clone" "$TOOLS_DIR/git-wt-clone"
fetch_file "git-wt-tools/git-wt-add" "$TOOLS_DIR/git-wt-add"
fetch_file "git-wt-tools/git-wt-rm" "$TOOLS_DIR/git-wt-rm"
fetch_file "git-wt-tools/git-wt-clean" "$TOOLS_DIR/git-wt-clean"
fetch_file "git-wt-tools/git-wt-status" "$TOOLS_DIR/git-wt-status"
fetch_file "git-wt-tools/devex-lib.sh" "$TOOLS_DIR/devex-lib.sh"

echo "Installing completion script to $TOOLS_DIR..."
fetch_file "git-wt-completion.sh" "$TOOLS_DIR/git-wt-completion.sh"

# 3. Make the scripts executable
chmod +x "$INSTALL_DIR/git-wt"
chmod +x "$TOOLS_DIR"/git-wt-*
chmod +x "$TOOLS_DIR/devex-lib.sh"

# 4. Optional: Install Git Aliases
echo -e "\n${YELLOW}Would you like to install the recommended Git shortcuts (e.g., 'git s' for status)?${NC}"
read -p "(y/n) " -n 1 -r
echo ""
if [[ $REPLY =~ ^[Yy]$ ]]; then
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
    fetch_file "auto-venv/auto-venv.sh" "$TOOLS_DIR/auto-venv.sh"
    
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
    # Setup PATH
    if ! grep -q '.local/bin' "$SHELL_RC"; then
        echo -e "\n${YELLOW}Adding $INSTALL_DIR to your PATH in $(basename "$SHELL_RC")...${NC}"
        echo "" >> "$SHELL_RC"
        echo '# >>> DevEx Manager PATH >>>' >> "$SHELL_RC"
        echo 'export PATH="$HOME/.local/bin:$PATH"' >> "$SHELL_RC"
        echo '# <<< DevEx Manager PATH <<<' >> "$SHELL_RC"
    else
        echo -e "\nPATH already configured in $(basename "$SHELL_RC")."
    fi

    # Setup Auto-completion
    if ! grep -q "git-wt-completion.sh" "$SHELL_RC"; then
        echo -e "${YELLOW}Adding auto-completion to $(basename "$SHELL_RC")...${NC}"
        
        echo "" >> "$SHELL_RC"
        echo '# >>> DevEx Manager Auto-completion >>>' >> "$SHELL_RC"
        
        # Zsh needs bashcompinit to run bash completion scripts properly
        if [[ "$SHELL" == */zsh ]]; then
            echo 'autoload -Uz compinit && compinit' >> "$SHELL_RC"
            echo 'autoload -Uz bashcompinit && bashcompinit' >> "$SHELL_RC"
        fi
        
        echo "source \"$TOOLS_DIR/git-wt-completion.sh\"" >> "$SHELL_RC"
        echo '# <<< DevEx Manager Auto-completion <<<' >> "$SHELL_RC"
    else
        echo -e "Auto-completion already configured in $(basename "$SHELL_RC")."
    fi

    # Setup Auto-Venv (if the user said yes)
    if [ "$INSTALL_AUTO_VENV" = true ]; then
        if ! grep -q "auto-venv.sh" "$SHELL_RC"; then
            echo -e "${YELLOW}Adding auto-venv to $(basename "$SHELL_RC")...${NC}"
            echo "" >> "$SHELL_RC"
            echo '# >>> DevEx Manager Auto-venv >>>' >> "$SHELL_RC"
            echo "source \"$TOOLS_DIR/auto-venv.sh\"" >> "$SHELL_RC"
            echo '# <<< DevEx Manager Auto-venv <<<' >> "$SHELL_RC"
        else
            echo -e "Auto-venv already configured in $(basename "$SHELL_RC")."
        fi
    fi
else
    echo -e "\n${YELLOW}Warning: Could not detect bash or zsh configuration file.${NC}"
    echo -e "Please ensure '$INSTALL_DIR' is in your PATH manually."
fi

echo -e "\n${GREEN}✓ Installation complete!${NC}"
echo -e "To start using your new commands, open a new terminal tab, or run:"
echo -e "  ${BLUE}source $SHELL_RC${NC}"
