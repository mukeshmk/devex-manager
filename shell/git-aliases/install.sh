#!/usr/bin/env bash

# Install recommended Git aliases

set -e

GREEN='\033[0;32m'
NC='\033[0m'

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
