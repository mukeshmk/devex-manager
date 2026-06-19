#!/usr/bin/env bash

# Bash completion for 'dx'
# Supports 'dx skills', 'dx nb', and 'dx ctx'

_dx() {
    local cur prev opts
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"

    # Main commands
    if [ $COMP_CWORD -eq 1 ]; then
        opts="skills nb ctx"
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
        ctx)
            local subcommands="show edit add done undo rm clean help todo"
            local todo_subcommands="add list done check undo uncheck rm delete clear"
            if [ $COMP_CWORD -eq 2 ]; then
                COMPREPLY=( $(compgen -W "$subcommands" -- "$cur") )
            elif [ $COMP_CWORD -eq 3 ]; then
                local prev_cmd="${COMP_WORDS[2]}"
                if [ "$prev_cmd" = "todo" ]; then
                    COMPREPLY=( $(compgen -W "$todo_subcommands" -- "$cur") )
                fi
            fi
            ;;
    esac
}
complete -F _dx dx
