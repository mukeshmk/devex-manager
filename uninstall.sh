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

echo -e "${BLUE}Uninstalling DevEx Manager...${NC}"

# 1. Remove installed files
echo -e "\n${YELLOW}Removing installed files...${NC}"

if [ -f "$INSTALL_DIR/git-wt" ]; then
    rm "$INSTALL_DIR/git-wt"
    echo "  ✓ Removed git-wt router"
else
    echo "  - git-wt router not found"
fi

if [ -d "$TOOLS_DIR" ]; then
    rm -rf "$TOOLS_DIR"
    echo "  ✓ Removed git-wt-tools directory"
else
    echo "  - git-wt-tools directory not found"
fi

# 2. Remove Git aliases
echo -e "\n${YELLOW}Would you like to remove the Git aliases installed by DevEx Manager?${NC}"
read -p "(y/n) " -n 1 -r
echo ""
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "Removing Git aliases..."
    git config --global --unset alias.a 2>/dev/null || true
    git config --global --unset alias.s 2>/dev/null || true
    git config --global --unset alias.d 2>/dev/null || true
    git config --global --unset alias.f 2>/dev/null || true
    git config --global --unset alias.m 2>/dev/null || true
    git config --global --unset alias.c 2>/dev/null || true
    git config --global --unset alias.b 2>/dev/null || true
    git config --global --unset alias.l 2>/dev/null || true
    git config --global --unset alias.r 2>/dev/null || true
    git config --global --unset alias.rs 2>/dev/null || true
    git config --global --unset alias.ls 2>/dev/null || true
    git config --global --unset alias.bn 2>/dev/null || true
    git config --global --unset alias.fet 2>/dev/null || true
    git config --global --unset alias.mer 2>/dev/null || true
    git config --global --unset alias.pul 2>/dev/null || true
    git config --global --unset alias.pus 2>/dev/null || true
    git config --global --unset alias.stas 2>/dev/null || true
    echo -e "${GREEN}✓ Git aliases removed${NC}"
else
    echo "Skipping Git aliases removal."
fi

# 3. Clean up shell configuration
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

if [ -n "$SHELL_RC" ] && [ -f "$SHELL_RC" ]; then
    echo -e "\n${YELLOW}Cleaning up shell configuration in $(basename "$SHELL_RC")...${NC}"
    
    # Create a backup
    cp "$SHELL_RC" "${SHELL_RC}.devex-backup"
    echo "  ✓ Created backup: ${SHELL_RC}.devex-backup"
    
    # Remove DevEx Manager related lines
    # This removes the comment lines and the actual configuration lines
    sed -i.tmp '/# Added by DevEx Manager [Ii]nstaller/d' "$SHELL_RC"
    sed -i.tmp '/export PATH=.*\.local\/bin.*PATH/d' "$SHELL_RC"
    sed -i.tmp '/git-wt-completion\.sh/d' "$SHELL_RC"
    sed -i.tmp '/auto-venv\.sh/d' "$SHELL_RC"
    
    # For zsh-specific lines added by the installer
    if [[ "$SHELL" == */zsh ]]; then
        # Remove compinit and bashcompinit lines that were added by DevEx Manager
        sed -i.tmp '/autoload -Uz compinit && compinit/d' "$SHELL_RC"
        sed -i.tmp '/autoload -Uz bashcompinit && bashcompinit/d' "$SHELL_RC"
    fi
    
    # Remove consecutive empty lines (collapse multiple blank lines into one)
    awk 'NF {blank=0} !NF {blank++} blank < 2' "$SHELL_RC" > "${SHELL_RC}.cleaned"
    mv "${SHELL_RC}.cleaned" "$SHELL_RC"
    
    # Clean up the temporary file
    rm -f "${SHELL_RC}.tmp"
    
    echo -e "${GREEN}✓ Shell configuration cleaned${NC}"
    echo -e "  If you want to restore the original, use: ${SHELL_RC}.devex-backup"
else
    echo -e "\n${YELLOW}Could not find shell configuration file to clean up.${NC}"
fi

echo -e "\n${GREEN}✓ Uninstallation complete!${NC}"
echo -e "\nTo complete the uninstallation:"
echo -e "  1. Open a new terminal tab, or run: ${BLUE}source $SHELL_RC${NC}"
echo -e "  2. Optionally remove the backup file: ${BLUE}rm ${SHELL_RC}.devex-backup${NC}"
echo -e "\nNote: The PATH entry for ~/.local/bin was NOT removed as it may be used by other tools."
