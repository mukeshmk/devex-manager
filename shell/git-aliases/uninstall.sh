#!/usr/bin/env bash

# Uninstall Git aliases installed by DevEx Manager

set -e

GREEN='\033[0;32m'
NC='\033[0m'

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
