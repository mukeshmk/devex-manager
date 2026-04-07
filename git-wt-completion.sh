#!/usr/bin/env bash

# Bash completion for 'git wt'
# Git completion looks for functions named _git_<subcommand>

_git_wt() {
    local subcommands="clone add rm clean list lock move prune repair unlock status"
    COMPREPLY=( $(compgen -W "$subcommands" -- "${COMP_WORDS[COMP_CWORD]}") )
}
