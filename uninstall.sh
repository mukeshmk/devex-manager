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

if [[ ! -f "$SCRIPT_DIR/install.sh" ]]; then
    IS_REMOTE=true
fi

echo -e "${BLUE}Uninstalling DevEx Manager...${NC}"
if [ "$IS_REMOTE" = true ]; then
    echo -e "${BLUE}Mode: Remote uninstallation${NC}"
fi

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
    if [ "$IS_REMOTE" = true ]; then
        curl -fsSL "$REPO_RAW_URL/git-aliases/uninstall-git-aliases.sh" | bash
    else
        bash "$SCRIPT_DIR/git-aliases/uninstall-git-aliases.sh"
    fi
else
    echo "Skipping Git aliases removal."
fi

# 4. Clean up shell configuration
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
    
    # Remove DevEx Manager related lines using unique markers
    # Remove PATH section (including preceding empty line if present)
    sed -i.tmp '/^$/{ N; /\n# >>> DevEx Manager PATH >>>/{ N; :a; /# <<< DevEx Manager PATH <<</!{ N; ba; }; d; }; P; D; }' "$SHELL_RC"
    sed -i.tmp '/# >>> DevEx Manager PATH >>>/,/# <<< DevEx Manager PATH <<</d' "$SHELL_RC"
    
    # Remove Auto-completion section (including preceding empty line if present)
    sed -i.tmp '/^$/{ N; /\n# >>> DevEx Manager Auto-completion >>>/{ N; :a; /# <<< DevEx Manager Auto-completion <<</!{ N; ba; }; d; }; P; D; }' "$SHELL_RC"
    sed -i.tmp '/# >>> DevEx Manager Auto-completion >>>/,/# <<< DevEx Manager Auto-completion <<</d' "$SHELL_RC"
    
    # Remove Auto-venv section (including preceding empty line if present)
    sed -i.tmp '/^$/{ N; /\n# >>> DevEx Manager Auto-venv >>>/{ N; :a; /# <<< DevEx Manager Auto-venv <<</!{ N; ba; }; d; }; P; D; }' "$SHELL_RC"
    sed -i.tmp '/# >>> DevEx Manager Auto-venv >>>/,/# <<< DevEx Manager Auto-venv <<</d' "$SHELL_RC"
    
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
