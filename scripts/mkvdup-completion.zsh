#compdef mkvdup mkvdup-canary

# Zsh completion for mkvdup
# Install to /usr/share/zsh/vendor-completions/_mkvdup

_mkvdup_create() {
    _arguments -s \
        '(-v --verbose)'{-v,--verbose}'[Enable verbose/debug output]' \
        '(-h --help)'{-h,--help}'[Show help]' \
        '--version[Show version]' \
        '--warn-threshold=[Minimum space savings percentage to avoid warning]:percentage' \
        '--quiet[Suppress the space savings warning]' \
        '--non-interactive[Do not prompt on codec mismatch]' \
        '1:MKV file:_files -g "*.mkv(-.)"' \
        '2:Source directory:_files -/' \
        '3:Output file:_files -g "*.mkvdup(-.)"' \
        '4::Display name'
}

_mkvdup_batch_create() {
    _arguments -s \
        '(-v --verbose)'{-v,--verbose}'[Enable verbose/debug output]' \
        '(-h --help)'{-h,--help}'[Show help]' \
        '--version[Show version]' \
        '--warn-threshold=[Minimum space savings percentage to avoid warning]:percentage' \
        '--quiet[Suppress the space savings warning]' \
        '1:Manifest file:_files -g "*.y(a|)ml(-.)"'
}

_mkvdup_probe() {
    _arguments -s \
        '(-v --verbose)'{-v,--verbose}'[Enable verbose/debug output]' \
        '(-h --help)'{-h,--help}'[Show help]' \
        '--version[Show version]' \
        '1:MKV file:_files -g "*.mkv(-.)"' \
        '*:Source directories:_files -/'
}

_mkvdup_mount() {
    _arguments -s \
        '(-v --verbose)'{-v,--verbose}'[Enable verbose/debug output]' \
        '(-h --help)'{-h,--help}'[Show help]' \
        '--version[Show version]' \
        '--allow-other[Allow other users to access the mount]' \
        '(-f --foreground)'{-f,--foreground}'[Run in foreground]' \
        '--config-dir[Treat config argument as directory of YAML files]' \
        '--pid-file=[Write daemon PID to file]:PID file:_files' \
        '--daemon-timeout=[Timeout waiting for daemon startup]:duration' \
        '--default-uid=[Default UID for files and directories]:UID' \
        '--default-gid=[Default GID for files and directories]:GID' \
        '--default-file-mode=[Default mode for files (octal)]:mode' \
        '--default-dir-mode=[Default mode for directories (octal)]:mode' \
        '--permissions-file=[Path to permissions file]:permissions file:_files' \
        '--no-source-watch[Disable source file monitoring]' \
        '--on-source-change=[Action on source change]:action:(warn disable checksum)' \
        '--source-watch-poll-interval=[Polling interval for source file changes]:duration' \
        '--source-read-timeout=[Read timeout for network FS sources]:duration' \
        '1:Mount point:_files -/' \
        '*:Config files:_files -g "*.y(a|)ml(-.)"'
}

_mkvdup_info() {
    _arguments -s \
        '(-v --verbose)'{-v,--verbose}'[Enable verbose/debug output]' \
        '(-h --help)'{-h,--help}'[Show help]' \
        '--version[Show version]' \
        '--hide-unused-files[Hide unused source files]' \
        '1:Dedup file:_files -g "*.mkvdup(-.)"'
}

_mkvdup_verify() {
    _arguments -s \
        '(-v --verbose)'{-v,--verbose}'[Enable verbose/debug output]' \
        '(-h --help)'{-h,--help}'[Show help]' \
        '--version[Show version]' \
        '1:Dedup file:_files -g "*.mkvdup(-.)"' \
        '2:Source directory:_files -/' \
        '3:Original MKV:_files -g "*.mkv(-.)"'
}

_mkvdup_extract() {
    _arguments -s \
        '(-v --verbose)'{-v,--verbose}'[Enable verbose/debug output]' \
        '(-h --help)'{-h,--help}'[Show help]' \
        '--version[Show version]' \
        '1:Dedup file:_files -g "*.mkvdup(-.)"' \
        '2:Source directory:_files -/' \
        '3:Output MKV:_files -g "*.mkv(-.)"'
}

_mkvdup_check() {
    _arguments -s \
        '(-v --verbose)'{-v,--verbose}'[Enable verbose/debug output]' \
        '(-h --help)'{-h,--help}'[Show help]' \
        '--version[Show version]' \
        '--source-checksums[Verify source file checksums]' \
        '1:Dedup file:_files -g "*.mkvdup(-.)"' \
        '2:Source directory:_files -/'
}

_mkvdup_validate() {
    _arguments -s \
        '(-v --verbose)'{-v,--verbose}'[Enable verbose/debug output]' \
        '(-h --help)'{-h,--help}'[Show help]' \
        '--version[Show version]' \
        '--config-dir[Treat config argument as directory of YAML files]' \
        '--deep[Verify dedup file headers and internal checksums]' \
        '--strict[Treat warnings as errors]' \
        '*:Config files:_files -g "*.y(a|)ml(-.)"'
}

_mkvdup_reload() {
    _arguments -s \
        '(-v --verbose)'{-v,--verbose}'[Enable verbose/debug output]' \
        '(-h --help)'{-h,--help}'[Show help]' \
        '--version[Show version]' \
        '(--pid)--pid-file=[PID file of the running daemon]:PID file:_files' \
        '(--pid-file)--pid=[PID of the running daemon]' \
        '--config-dir[Treat config argument as directory of YAML files]' \
        '*:Config files:_files -g "*.y(a|)ml(-.)"'
}

_mkvdup_parse_mkv() {
    _arguments -s \
        '(-v --verbose)'{-v,--verbose}'[Enable verbose/debug output]' \
        '(-h --help)'{-h,--help}'[Show help]' \
        '--version[Show version]' \
        '1:MKV file:_files -g "*.mkv(-.)"'
}

_mkvdup_index_source() {
    _arguments -s \
        '(-v --verbose)'{-v,--verbose}'[Enable verbose/debug output]' \
        '(-h --help)'{-h,--help}'[Show help]' \
        '--version[Show version]' \
        '1:Source directory:_files -/'
}

_mkvdup_match() {
    _arguments -s \
        '(-v --verbose)'{-v,--verbose}'[Enable verbose/debug output]' \
        '(-h --help)'{-h,--help}'[Show help]' \
        '--version[Show version]' \
        '1:MKV file:_files -g "*.mkv(-.)"' \
        '2:Source directory:_files -/'
}

_mkvdup_deltadiag() {
    _arguments -s \
        '(-v --verbose)'{-v,--verbose}'[Enable verbose/debug output]' \
        '(-h --help)'{-h,--help}'[Show help]' \
        '--version[Show version]' \
        '1:Dedup file:_files -g "*.mkvdup(-.)"' \
        '2:MKV file:_files -g "*.mkv(-.)"'
}

_mkvdup() {
    local curcontext="$curcontext" state line
    typeset -A opt_args

    _arguments -C \
        '(-v --verbose)'{-v,--verbose}'[Enable verbose/debug output]' \
        '(-h --help)'{-h,--help}'[Show help]' \
        '--version[Show version]' \
        '1:command:->command' \
        '*::arg:->args'

    case $state in
        command)
            local -a subcommands
            subcommands=(
                'create:Create a dedup file from an MKV and its source directory'
                'batch-create:Create multiple dedup files from a manifest'
                'probe:Quick test if an MKV likely matches a source'
                'mount:Mount virtual filesystem from config files'
                'info:Show information about a dedup file'
                'verify:Verify a dedup file against the original MKV'
                'extract:Rebuild original MKV from dedup + source'
                'check:Check integrity of a dedup file and its source files'
                'validate:Validate configuration files for correctness'
                'reload:Reload a running daemon configuration'
                'parse-mkv:Parse and display MKV structure (debug)'
                'index-source:Index a source directory (debug)'
                'match:Match packets between MKV and source (debug)'
                'deltadiag:Analyze unmatched regions by stream type (debug)'
                'help:Show help for a command'
            )
            _describe -t commands 'mkvdup command' subcommands
            ;;
        args)
            case ${line[1]} in
                create)       _mkvdup_create ;;
                batch-create) _mkvdup_batch_create ;;
                probe)        _mkvdup_probe ;;
                mount)        _mkvdup_mount ;;
                info)         _mkvdup_info ;;
                verify)       _mkvdup_verify ;;
                extract)      _mkvdup_extract ;;
                check)        _mkvdup_check ;;
                validate)     _mkvdup_validate ;;
                reload)       _mkvdup_reload ;;
                parse-mkv)    _mkvdup_parse_mkv ;;
                index-source) _mkvdup_index_source ;;
                match)        _mkvdup_match ;;
                deltadiag)    _mkvdup_deltadiag ;;
                help)
                    local -a help_cmds
                    help_cmds=(create batch-create probe mount info verify extract check validate reload parse-mkv index-source match deltadiag)
                    _describe -t commands 'command' help_cmds
                    ;;
            esac
            ;;
    esac
}

_mkvdup "$@"
