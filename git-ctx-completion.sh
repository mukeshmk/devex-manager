#!/usr/bin/env bash

# Bash/Zsh completion for 'git ctx'
# Git's autocomplete framework looks for functions named _git_<subcommand>

_git_ctx() {
    local cur prev
    cur="${COMP_WORDS[COMP_CWORD]}"
    
    local subcommands="show edit add done undo rm clean help todo"
    local todo_subcommands="add list done check undo uncheck rm delete clear"

    # Find the index of 'ctx' in the command words list
    local ctx_index=-1
    for i in "${!COMP_WORDS[@]}"; do
        if [[ "${COMP_WORDS[i]}" == "ctx" ]]; then
            ctx_index=$i
            break
        fi
    done

    if [ $ctx_index -eq -1 ]; then
        return
    fi

    local cmd_word_index=$((ctx_index + 1))
    local subcmd_word_index=$((ctx_index + 2))

    if [ $COMP_CWORD -eq $cmd_word_index ]; then
        # Completing the main subcommand of git ctx
        COMPREPLY=( $(compgen -W "$subcommands" -- "$cur") )
    elif [ $COMP_CWORD -eq $subcmd_word_index ]; then
        local prev_cmd="${COMP_WORDS[cmd_word_index]}"
        if [ "$prev_cmd" = "todo" ]; then
            # Completing todo sub-commands
            COMPREPLY=( $(compgen -W "$todo_subcommands" -- "$cur") )
        fi
    fi
}
