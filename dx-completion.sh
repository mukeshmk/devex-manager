#!/usr/bin/env bash

# Bash completion for 'dx'
# Supports 'dx skills' and 'dx nb'

_dx() {
    local cur prev opts
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"

    # Main commands
    if [ $COMP_CWORD -eq 1 ]; then
        opts="skills nb"
        COMPREPLY=( $(compgen -W "$opts" -- "$cur") )
        return 0
    fi

    # Subcommands
    case "${COMP_WORDS[1]}" in
        skills)
            local subcommands="list sync diff edit rm"
            COMPREPLY=( $(compgen -W "$subcommands" -- "$cur") )
            ;;
        nb)
            local subcommands="strip diff kernel list"
            COMPREPLY=( $(compgen -W "$subcommands" -- "$cur") )
            ;;
    esac
}
complete -F _dx dx
