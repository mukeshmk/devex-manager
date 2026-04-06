#!/usr/bin/env bash

# Bash completion for 'git nb'
# Git completion looks for functions named _git_<subcommand>

_git_nb() {
    local subcommands="strip diff kernel list"
    COMPREPLY=( $(compgen -W "$subcommands" -- "${COMP_WORDS[COMP_CWORD]}") )
}
