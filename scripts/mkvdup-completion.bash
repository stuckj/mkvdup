# Bash completion for mkvdup
# Install to /usr/share/bash-completion/completions/mkvdup

_mkvdup() {
    local cur prev words cword

    # Fallback when bash-completion package is not installed
    if ! type _init_completion &>/dev/null; then
        COMPREPLY=()
        cur="${COMP_WORDS[COMP_CWORD]}"
        prev="${COMP_WORDS[COMP_CWORD-1]}"
        words=("${COMP_WORDS[@]}")
        cword=$COMP_CWORD
    else
        _init_completion || return
    fi

    local commands="create probe mount info verify parse-mkv index-source match help"
    local global_opts="-v --verbose -h --help --version"

    # Find the command (first non-option argument after mkvdup)
    local cmd=""
    local i
    for ((i=1; i < cword; i++)); do
        case "${words[i]}" in
            -v|--verbose|-h|--help|--version)
                ;;
            -*)
                # Skip unknown options and their potential arguments
                ;;
            *)
                cmd="${words[i]}"
                break
                ;;
        esac
    done

    # If no command yet, complete commands and global options
    if [[ -z "$cmd" ]]; then
        if [[ "$cur" == -* ]]; then
            COMPREPLY=($(compgen -W "$global_opts" -- "$cur"))
        else
            COMPREPLY=($(compgen -W "$commands" -- "$cur"))
        fi
        return
    fi

    # Command-specific completions
    case "$cmd" in
        create)
            # create <mkv-file> <source-dir> [output] [name]
            if [[ "$cur" == -* ]]; then
                COMPREPLY=($(compgen -W "$global_opts" -- "$cur"))
                return
            fi
            _filedir
            ;;

        probe)
            # probe <mkv-file> <source-dir>...
            if [[ "$cur" == -* ]]; then
                COMPREPLY=($(compgen -W "$global_opts" -- "$cur"))
                return
            fi
            _filedir
            ;;

        mount)
            # mount [options] <mountpoint> [config.yaml...]
            local mount_opts="--allow-other --foreground -f --config-dir --pid-file --daemon-timeout --default-uid --default-gid --default-file-mode --default-dir-mode --permissions-file"

            if [[ "$cur" == -* ]]; then
                COMPREPLY=($(compgen -W "$mount_opts $global_opts" -- "$cur"))
                return
            fi

            case "$prev" in
                --pid-file|--permissions-file)
                    # Complete any file path
                    _filedir
                    return
                    ;;
                --daemon-timeout)
                    # Suggest common timeout values
                    COMPREPLY=($(compgen -W "10s 30s 60s 2m 5m" -- "$cur"))
                    return
                    ;;
                --default-uid|--default-gid|--default-file-mode|--default-dir-mode)
                    # Numeric values; no useful completion to offer
                    return
                    ;;
            esac

            # Complete directories for mountpoint, or yaml files for config
            if [[ "$cur" == *.yaml || "$cur" == *.yml ]]; then
                _filedir '@(yaml|yml)'
            else
                _filedir
            fi
            ;;

        info)
            # info <dedup-file>
            if [[ "$cur" == -* ]]; then
                COMPREPLY=($(compgen -W "$global_opts" -- "$cur"))
                return
            fi
            _filedir '@(mkvdup)'
            ;;

        verify)
            # verify <dedup-file> <source-dir> <original-mkv>
            if [[ "$cur" == -* ]]; then
                COMPREPLY=($(compgen -W "$global_opts" -- "$cur"))
                return
            fi
            _filedir
            ;;

        parse-mkv)
            # parse-mkv <mkv-file>
            if [[ "$cur" == -* ]]; then
                COMPREPLY=($(compgen -W "$global_opts" -- "$cur"))
                return
            fi
            _filedir '@(mkv|MKV)'
            ;;

        index-source)
            # index-source <source-dir>
            if [[ "$cur" == -* ]]; then
                COMPREPLY=($(compgen -W "$global_opts" -- "$cur"))
                return
            fi
            _filedir -d
            ;;

        match)
            # match <mkv-file> <source-dir>
            if [[ "$cur" == -* ]]; then
                COMPREPLY=($(compgen -W "$global_opts" -- "$cur"))
                return
            fi
            _filedir
            ;;

        help)
            # help [command]
            COMPREPLY=($(compgen -W "$commands" -- "$cur"))
            ;;
    esac
}

complete -F _mkvdup mkvdup
