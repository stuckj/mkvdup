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

    # Minimal _filedir fallback when bash-completion is not available.
    # Handles plain calls, "-d" for directories, and "@(ext|ext)" patterns.
    if ! type _filedir &>/dev/null; then
        _filedir() {
            local IFS=$'\n'
            if [[ "$1" == "-d" ]]; then
                COMPREPLY=($(compgen -d -- "$cur"))
            elif [[ "$1" == @\(*\) ]]; then
                # Extract extensions from @(ext1|ext2) pattern
                local exts="${1#@(}"
                exts="${exts%)}"
                local -a results=()
                local ext
                while IFS='|' read -ra parts; do
                    for ext in "${parts[@]}"; do
                        results+=($(compgen -f -X "!*.$ext" -- "$cur"))
                    done
                done <<< "$exts"
                # Also include directories for navigation
                results+=($(compgen -d -- "$cur"))
                COMPREPLY=("${results[@]}")
            else
                COMPREPLY=($(compgen -f -- "$cur"))
            fi
        }
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

    # Global options available for all commands when typing -<TAB>
    if [[ "$cur" == -* && "$cmd" != "mount" ]]; then
        COMPREPLY=($(compgen -W "$global_opts" -- "$cur"))
        return
    fi

    # Command-specific completions
    case "$cmd" in
        create)
            # create <mkv-file> <source-dir> [output] [name]
            _filedir
            ;;

        probe)
            # probe <mkv-file> <source-dir>...
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
            _filedir '@(mkvdup)'
            ;;

        verify)
            # verify <dedup-file> <source-dir> <original-mkv>
            _filedir
            ;;

        parse-mkv)
            # parse-mkv <mkv-file>
            _filedir '@(mkv|MKV)'
            ;;

        index-source)
            # index-source <source-dir>
            _filedir -d
            ;;

        match)
            # match <mkv-file> <source-dir>
            _filedir
            ;;

        help)
            # help [command]
            COMPREPLY=($(compgen -W "$commands" -- "$cur"))
            ;;
    esac
}

complete -F _mkvdup mkvdup
