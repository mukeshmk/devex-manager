#!/usr/bin/env bash

set -e # Exit on error

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

INSTALL_DIR="$HOME/.local/bin"
TOOLS_DIR="$INSTALL_DIR/git-wt-tools"

echo -e "${BLUE}Installing DevEx Manager (git wt)...${NC}"

# 1. Create the destination directories
mkdir -p "$INSTALL_DIR"
mkdir -p "$TOOLS_DIR"

# 2. Get the directory where the install script is currently located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "Copying router to $INSTALL_DIR..."
cp "$SCRIPT_DIR/git-wt" "$INSTALL_DIR/"

echo "Copying sub-commands to $TOOLS_DIR..."
cp "$SCRIPT_DIR/git-wt-tools/git-wt-clone" "$TOOLS_DIR/"
cp "$SCRIPT_DIR/git-wt-tools/git-wt-add" "$TOOLS_DIR/"
cp "$SCRIPT_DIR/git-wt-tools/git-wt-rm" "$TOOLS_DIR/"
cp "$SCRIPT_DIR/git-wt-tools/devex-lib.sh" "$TOOLS_DIR/"

echo "Copying completion script to $TOOLS_DIR..."
cp "$SCRIPT_DIR/git-wt-completion.sh" "$TOOLS_DIR/"

# 3. Make the scripts executable (completion script doesn't need to be executable, just sourced)
chmod +x "$INSTALL_DIR/git-wt"
chmod +x "$TOOLS_DIR"/git-wt-*
chmod +x "$TOOLS_DIR/devex-lib.sh"

# 4. Optional: Install Git Aliases
echo -e "\n${YELLOW}Would you like to install the recommended Git shortcuts (e.g., 'git s' for status)?${NC}"
read -p "(y/n) " -n 1 -r
echo "" 
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "Configuring Git aliases..."
    git config --global alias.a "add"
    git config --global alias.s "status"
    git config --global alias.d "diff"
    git config --global alias.f "fetch"
    git config --global alias.m "merge"
    git config --global alias.c "checkout"
    git config --global alias.b "branch"
    git config --global alias.l "log"
    git config --global alias.r "restore"
    git config --global alias.rs "restore --staged"
    git config --global alias.ls "log --stat"
    git config --global alias.bn "rev-parse --abbrev-ref HEAD"
    
    git config --global alias.fet '!git fetch origin $(git bn)'
    git config --global alias.mer 'merge @{u}'
    git config --global alias.pul '!git pull origin $(git bn)'
    git config --global alias.pus '!git push origin $(git bn)'
    git config --global alias.stas '!git stash push -p'
    
    echo -e "${GREEN}✓ Git aliases installed successfully!${NC}"
else
    echo "Skipping Git aliases."
fi

# 5. Optional: Install Python Auto-Venv Tools
echo -e "\n${YELLOW}Would you like to install the Python auto-venv tools (auto-activates .venv when you cd into a directory)?${NC}"
read -p "(y/n) " -n 1 -r
echo ""
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "Installing auto-venv script..."
    cp "$SCRIPT_DIR/auto-venv/auto-venv.sh" "$TOOLS_DIR/"
    
    # We will source this file in the shell config step below
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
