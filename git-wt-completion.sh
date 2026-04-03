#!/usr/bin/env bash

# Auto-completion logic for 'git wt'
# Git's master completion script automatically looks for functions named _git_<subcommand>

_git_wt() {
    # The list of all available commands
    local subcommands="clone add clean list lock move prune repair unlock status"
    
    # compgen is a built-in bash tool that filters the subcommands based on what the user typed so far
    COMPREPLY=( $(compgen -W "$subcommands" -- "${COMP_WORDS[COMP_CWORD]}") )
}
