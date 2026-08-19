# bash completion for thing
# Install: source this file from your ~/.bashrc, or drop it into a
#          bash_completion.d directory (e.g. "$(brew --prefix)/etc/bash_completion.d/"
#          on Homebrew, or /etc/bash_completion.d/ on most Linux distros).

# When bash-completion is not loaded, provide a minimal _filedir so file and
# directory completion still degrades gracefully instead of erroring.
if ! declare -F _filedir >/dev/null 2>&1; then
    _filedir() {
        if [[ "$1" == -d ]]; then
            COMPREPLY=($(compgen -d -- "$cur"))
        else
            COMPREPLY=($(compgen -f -- "$cur"))
        fi
    }
fi

_thing() {
    local cur prev words cword
    _init_completion 2>/dev/null || {
        cur="${COMP_WORDS[COMP_CWORD]}"
        prev="${COMP_WORDS[COMP_CWORD-1]}"
        words=("${COMP_WORDS[@]}")
        cword=$COMP_CWORD
    }

    local commands="init add ls show status priority mv rm archive unarchive link find check tree export import server help --version"
    local link_verbs="add rm list"
    local server_verbs="start stop restart status logs"
    local statuses="todo doing done paused"
    local priorities="high medium low"
    local global_flags="--data-dir --config -g --global"

    # Complete flag values regardless of position.
    case "$prev" in
        --data-dir|--config) _filedir -d; return ;;
        --priority) COMPREPLY=($(compgen -W "$priorities" -- "$cur")); return ;;
    esac

    # Top-level command.
    if [[ $cword -eq 1 ]]; then
        COMPREPLY=($(compgen -W "$commands" -- "$cur"))
        return
    fi

    local cmd="${words[1]}"
    case "$cmd" in
        status)
            # thing status <ref> <status>
            [[ $cword -eq 3 && "$cur" != -* ]] && COMPREPLY=($(compgen -W "$statuses" -- "$cur"))
            [[ "$cur" == -* ]] && COMPREPLY=($(compgen -W "$global_flags" -- "$cur"))
            ;;
        priority)
            # thing priority <ref> <priority>
            [[ $cword -eq 3 && "$cur" != -* ]] && COMPREPLY=($(compgen -W "$priorities" -- "$cur"))
            [[ "$cur" == -* ]] && COMPREPLY=($(compgen -W "$global_flags" -- "$cur"))
            ;;
        add)
            [[ "$cur" == -* ]] && COMPREPLY=($(compgen -W "$global_flags --category --priority --tags" -- "$cur"))
            ;;
        link)
            if [[ $cword -eq 2 ]]; then
                COMPREPLY=($(compgen -W "$link_verbs" -- "$cur"))
                return
            fi
            [[ "$cur" == -* ]] && COMPREPLY=($(compgen -W "$global_flags --label" -- "$cur"))
            ;;
        find)
            [[ "$cur" == -* ]] && COMPREPLY=($(compgen -W "$global_flags --json" -- "$cur"))
            ;;
        ls)
            [[ "$cur" == -* ]] && COMPREPLY=($(compgen -W "$global_flags --archived --all" -- "$cur"))
            ;;
        unarchive)
            [[ "$cur" == -* ]] && COMPREPLY=($(compgen -W "$global_flags --to" -- "$cur"))
            ;;
        import)
            if [[ "$cur" == -* ]]; then
                COMPREPLY=($(compgen -W "$global_flags --dry-run" -- "$cur"))
            else
                _filedir json
            fi
            ;;
        server)
            if [[ $cword -eq 2 ]]; then
                COMPREPLY=($(compgen -W "$server_verbs" -- "$cur"))
                return
            fi
            # Per-subcommand flags.
            local sub="${words[2]}"
            case "$sub" in
                start)   [[ "$cur" == -* ]] && COMPREPLY=($(compgen -W "--port --open" -- "$cur")) ;;
                restart) [[ "$cur" == -* ]] && COMPREPLY=($(compgen -W "--port" -- "$cur")) ;;
                logs)    [[ "$cur" == -* ]] && COMPREPLY=($(compgen -W "-f --follow -n --lines" -- "$cur")) ;;
            esac
            ;;
        *)
            # show, mv, rm, archive, init, check, tree, export: global flags only.
            [[ "$cur" == -* ]] && COMPREPLY=($(compgen -W "$global_flags" -- "$cur"))
            ;;
    esac
}

complete -F _thing thing
