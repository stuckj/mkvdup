# Bash completion for mkvdup
# Install to /usr/share/bash-completion/completions/mkvdup

_mkvdup() {
    local cur prev words cword
    _init_completion || return

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
            _filedir
            ;;

        probe)
            # probe <mkv-file> <source-dir>...
            _filedir
            ;;

        mount)
            # mount [options] <mountpoint> [config.yaml...]
            local mount_opts="--allow-other --foreground -f --config-dir --pid-file --daemon-timeout"

            if [[ "$cur" == -* ]]; then
                COMPREPLY=($(compgen -W "$mount_opts" -- "$cur"))
                return
            fi

            case "$prev" in
                --pid-file)
                    # Complete any file path
                    _filedir
                    return
                    ;;
                --daemon-timeout)
                    # Suggest common timeout values
                    COMPREPLY=($(compgen -W "10s 30s 60s 2m 5m" -- "$cur"))
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
