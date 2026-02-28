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
    # Uses mapfile to avoid pathname expansion on glob metacharacters in filenames.
    if ! type _filedir &>/dev/null; then
        _filedir() {
            local -a _tmp
            if [[ "$1" == "-d" ]]; then
                mapfile -t COMPREPLY < <(compgen -d -- "$cur")
            elif [[ "$1" == @\(*\) ]]; then
                # Extract extensions from @(ext1|ext2) pattern
                local exts="${1#@(}"
                exts="${exts%)}"
                local -a results=()
                local ext
                while IFS='|' read -ra parts; do
                    for ext in "${parts[@]}"; do
                        mapfile -t _tmp < <(compgen -f -X "!*.$ext" -- "$cur")
                        results+=("${_tmp[@]}")
                    done
                done <<< "$exts"
                # Also include directories for navigation
                mapfile -t _tmp < <(compgen -d -- "$cur")
                results+=("${_tmp[@]}")
                COMPREPLY=("${results[@]}")
            else
                mapfile -t COMPREPLY < <(compgen -f -- "$cur")
            fi
        }
    fi

    local commands="create batch-create probe mount info verify extract check stats validate reload parse-mkv index-source match deltadiag help"
    local global_opts="-v --verbose -q --quiet --no-progress --log-file --log-verbose -h --help --version"

    # Find the command (first non-option argument after mkvdup)
    local cmd=""
    local i
    for ((i=1; i < cword; i++)); do
        case "${words[i]}" in
            -v|--verbose|-q|--quiet|--no-progress|--log-verbose|-h|--help|--version)
                ;;
            --log-file)
                # Skip --log-file and its argument
                ((i++))
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
        case "$prev" in
            --log-file)
                _filedir
                return
                ;;
        esac
        if [[ "$cur" == -* ]]; then
            COMPREPLY=($(compgen -W "$global_opts" -- "$cur"))
        else
            COMPREPLY=($(compgen -W "$commands" -- "$cur"))
        fi
        return
    fi

    # Global option --log-file takes a path argument
    if [[ "$prev" == "--log-file" ]]; then
        _filedir
        return
    fi

    # Global options available for commands that don't define their own options
    if [[ "$cur" == -* && "$cmd" != "create" && "$cmd" != "batch-create" && "$cmd" != "mount" && "$cmd" != "check" && "$cmd" != "stats" && "$cmd" != "validate" && "$cmd" != "reload" && "$cmd" != "info" ]]; then
        COMPREPLY=($(compgen -W "$global_opts" -- "$cur"))
        return
    fi

    # Command-specific completions
    case "$cmd" in
        create)
            # create [options] <mkv-file> <source-dir> [output] [name]
            local create_opts="--warn-threshold --non-interactive"
            if [[ "$cur" == -* ]]; then
                COMPREPLY=($(compgen -W "$create_opts $global_opts" -- "$cur"))
                return
            fi
            case "$prev" in
                --warn-threshold)
                    # Numeric value; no useful completion to offer
                    return
                    ;;
            esac
            _filedir
            ;;

        batch-create)
            # batch-create [options] <manifest.yaml>
            local batch_create_opts="--warn-threshold --skip-codec-mismatch"
            if [[ "$cur" == -* ]]; then
                COMPREPLY=($(compgen -W "$batch_create_opts $global_opts" -- "$cur"))
                return
            fi
            case "$prev" in
                --warn-threshold)
                    # Numeric value; no useful completion to offer
                    return
                    ;;
            esac
            _filedir '@(yaml|yml)'
            ;;

        probe)
            # probe <mkv-file> <source-dir>...
            _filedir
            ;;

        mount)
            # mount [options] <mountpoint> [config.yaml...]
            local mount_opts="--allow-other --foreground -f --config-dir --pid-file --daemon-timeout --default-uid --default-gid --default-file-mode --default-dir-mode --permissions-file --no-source-watch --on-source-change --source-watch-poll-interval --source-read-timeout"

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
                --on-source-change)
                    COMPREPLY=($(compgen -W "warn disable checksum" -- "$cur"))
                    return
                    ;;
                --source-watch-poll-interval|--source-read-timeout)
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
            # info [--hide-unused-files] <dedup-file>
            local info_opts="--hide-unused-files"
            if [[ "$cur" == -* ]]; then
                COMPREPLY=($(compgen -W "$info_opts $global_opts" -- "$cur"))
            else
                _filedir '@(mkvdup)'
            fi
            ;;

        verify)
            # verify <dedup-file> <source-dir> <original-mkv>
            _filedir
            ;;

        extract)
            # extract <dedup-file> <source-dir> <output-mkv>
            _filedir
            ;;

        check)
            # check <dedup-file> <source-dir> [--source-checksums]
            local check_opts="--source-checksums"
            if [[ "$cur" == -* ]]; then
                COMPREPLY=($(compgen -W "$check_opts $global_opts" -- "$cur"))
                return
            fi
            _filedir
            ;;

        stats)
            # stats [options] <config.yaml...>
            local stats_opts="--config-dir"
            if [[ "$cur" == -* ]]; then
                COMPREPLY=($(compgen -W "$stats_opts $global_opts" -- "$cur"))
                return
            fi
            if [[ "$cur" == *.yaml || "$cur" == *.yml ]]; then
                _filedir '@(yaml|yml)'
            else
                _filedir
            fi
            ;;

        validate)
            # validate [options] [config.yaml...]
            local validate_opts="--config-dir --deep --strict"
            if [[ "$cur" == -* ]]; then
                COMPREPLY=($(compgen -W "$validate_opts $global_opts" -- "$cur"))
                return
            fi
            case "$prev" in
                --config-dir)
                    _filedir -d
                    return
                    ;;
            esac
            # Allow any file (default config is .conf, not just .yaml/.yml)
            if [[ "$cur" == *.yaml || "$cur" == *.yml ]]; then
                _filedir '@(yaml|yml)'
            else
                _filedir
            fi
            ;;

        reload)
            # reload {--pid-file PATH | --pid PID} [options] [config.yaml...]
            local reload_opts="--pid-file --pid --config-dir"
            if [[ "$cur" == -* ]]; then
                COMPREPLY=($(compgen -W "$reload_opts $global_opts" -- "$cur"))
                return
            fi
            case "$prev" in
                --pid-file)
                    _filedir
                    return
                    ;;
                --pid)
                    # PID argument — no file completion
                    return
                    ;;
            esac
            _filedir '@(yaml|yml)'
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

        deltadiag)
            # deltadiag <dedup-file> <mkv-file>
            _filedir
            ;;

        help)
            # help [command]
            COMPREPLY=($(compgen -W "$commands" -- "$cur"))
            ;;
    esac
}

# Register completion for mkvdup (and mkvdup-canary if present).
# Explicit names avoid issues when the script is sourced from a path
# where the filename doesn't match the command (e.g., mkvdup-completion.bash).
complete -F _mkvdup mkvdup
complete -F _mkvdup mkvdup-canary
