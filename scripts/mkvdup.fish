# Fish completion for mkvdup
# Install to /usr/share/fish/vendor_completions.d/mkvdup.fish

# Helper function to check if a subcommand has been given.
# Only examines completed tokens (excludes the current token being typed)
# so that partial input like "batch-" still offers "batch-create".
function __fish_mkvdup_needs_command
    set -l tokens (commandline -opc)
    set -l skip_next 0
    # Examine all tokens except the last (which is being completed)
    for i in $tokens[2..-2]
        if test $skip_next -eq 1
            set skip_next 0
            continue
        end
        switch $i
            case '-v' '--verbose' '-q' '--quiet' '--no-progress' '-h' '--help' '--version'
                continue
            case '--log-file'
                set skip_next 1
                continue
            case '-*'
                continue
            case '*'
                return 1
        end
    end
    return 0
end

function __fish_mkvdup_using_command
    set -l tokens (commandline -opc)
    set -l target $argv[1]
    set -l skip_next 0
    for i in $tokens[2..-1]
        if test $skip_next -eq 1
            set skip_next 0
            continue
        end
        switch $i
            case '-v' '--verbose' '-q' '--quiet' '--no-progress' '-h' '--help' '--version'
                continue
            case '--log-file'
                set skip_next 1
                continue
            case '-*'
                continue
            case $target
                return 0
            case '*'
                return 1
        end
    end
    return 1
end

# Register completions for both mkvdup and mkvdup-canary
for cmd in mkvdup mkvdup-canary

# Disable file completions by default (we'll enable them per-subcommand)
complete -c $cmd -f

# Global options
complete -c $cmd -n __fish_mkvdup_needs_command -s v -l verbose -d 'Enable verbose/debug output'
complete -c $cmd -n __fish_mkvdup_needs_command -s q -l quiet -d 'Suppress informational progress output'
complete -c $cmd -n __fish_mkvdup_needs_command -l no-progress -d 'Disable progress bars'
complete -c $cmd -n __fish_mkvdup_needs_command -l log-file -d 'Duplicate output to a log file' -r
complete -c $cmd -n __fish_mkvdup_needs_command -s h -l help -d 'Show help'
complete -c $cmd -n __fish_mkvdup_needs_command -l version -d 'Show version'

# Subcommands
complete -c $cmd -n __fish_mkvdup_needs_command -a create -d 'Create a dedup file from an MKV and its source directory'
complete -c $cmd -n __fish_mkvdup_needs_command -a batch-create -d 'Create multiple dedup files from a manifest'
complete -c $cmd -n __fish_mkvdup_needs_command -a probe -d 'Quick test if MKV file(s) likely match source(s)'
complete -c $cmd -n __fish_mkvdup_needs_command -a mount -d 'Mount virtual filesystem from config files'
complete -c $cmd -n __fish_mkvdup_needs_command -a info -d 'Show information about a dedup file'
complete -c $cmd -n __fish_mkvdup_needs_command -a verify -d 'Verify a dedup file against the original MKV'
complete -c $cmd -n __fish_mkvdup_needs_command -a extract -d 'Rebuild original MKV from dedup + source'
complete -c $cmd -n __fish_mkvdup_needs_command -a check -d 'Check integrity of a dedup file and its source files'
complete -c $cmd -n __fish_mkvdup_needs_command -a stats -d 'Show space savings and file statistics'
complete -c $cmd -n __fish_mkvdup_needs_command -a validate -d 'Validate configuration files for correctness'
complete -c $cmd -n __fish_mkvdup_needs_command -a reload -d 'Reload a running daemon configuration'
complete -c $cmd -n __fish_mkvdup_needs_command -a parse-mkv -d 'Parse and display MKV structure (debug)'
complete -c $cmd -n __fish_mkvdup_needs_command -a index-source -d 'Index a source directory (debug)'
complete -c $cmd -n __fish_mkvdup_needs_command -a match -d 'Match packets between MKV and source (debug)'
complete -c $cmd -n __fish_mkvdup_needs_command -a deltadiag -d 'Analyze delta entries in a dedup file'
complete -c $cmd -n __fish_mkvdup_needs_command -a help -d 'Show help for a command'

# create options
complete -c $cmd -n '__fish_mkvdup_using_command create' -s v -l verbose -d 'Enable verbose/debug output'
complete -c $cmd -n '__fish_mkvdup_using_command create' -s q -l quiet -d 'Suppress informational progress output'
complete -c $cmd -n '__fish_mkvdup_using_command create' -l no-progress -d 'Disable progress bars'
complete -c $cmd -n '__fish_mkvdup_using_command create' -l warn-threshold -d 'Minimum space savings percentage' -x
complete -c $cmd -n '__fish_mkvdup_using_command create' -l non-interactive -d 'Do not prompt on codec mismatch'
complete -c $cmd -n '__fish_mkvdup_using_command create' -F -d 'MKV file or source directory'

# batch-create options
complete -c $cmd -n '__fish_mkvdup_using_command batch-create' -s v -l verbose -d 'Enable verbose/debug output'
complete -c $cmd -n '__fish_mkvdup_using_command batch-create' -s q -l quiet -d 'Suppress informational progress output'
complete -c $cmd -n '__fish_mkvdup_using_command batch-create' -l no-progress -d 'Disable progress bars'
complete -c $cmd -n '__fish_mkvdup_using_command batch-create' -l warn-threshold -d 'Minimum space savings percentage' -x
complete -c $cmd -n '__fish_mkvdup_using_command batch-create' -l skip-codec-mismatch -d 'Skip MKVs with codec mismatch'
complete -c $cmd -n '__fish_mkvdup_using_command batch-create' -F -d 'Manifest file'

# probe options
complete -c $cmd -n '__fish_mkvdup_using_command probe' -F -d 'MKV files, --, and source directories'

# mount options
complete -c $cmd -n '__fish_mkvdup_using_command mount' -l allow-other -d 'Allow other users to access the mount'
complete -c $cmd -n '__fish_mkvdup_using_command mount' -s f -l foreground -d 'Run in foreground'
complete -c $cmd -n '__fish_mkvdup_using_command mount' -l config-dir -d 'Treat config argument as directory of YAML files'
complete -c $cmd -n '__fish_mkvdup_using_command mount' -l pid-file -d 'Write daemon PID to file' -r
complete -c $cmd -n '__fish_mkvdup_using_command mount' -l daemon-timeout -d 'Timeout waiting for daemon startup' -x
complete -c $cmd -n '__fish_mkvdup_using_command mount' -l default-uid -d 'Default UID for files and directories' -x
complete -c $cmd -n '__fish_mkvdup_using_command mount' -l default-gid -d 'Default GID for files and directories' -x
complete -c $cmd -n '__fish_mkvdup_using_command mount' -l default-file-mode -d 'Default mode for files (octal)' -x
complete -c $cmd -n '__fish_mkvdup_using_command mount' -l default-dir-mode -d 'Default mode for directories (octal)' -x
complete -c $cmd -n '__fish_mkvdup_using_command mount' -l permissions-file -d 'Path to permissions file' -r
complete -c $cmd -n '__fish_mkvdup_using_command mount' -l no-source-watch -d 'Disable source file monitoring'
complete -c $cmd -n '__fish_mkvdup_using_command mount' -l on-source-change -d 'Action on source change' -x -a 'warn disable checksum'
complete -c $cmd -n '__fish_mkvdup_using_command mount' -l source-watch-poll-interval -d 'Polling interval for source file changes' -x
complete -c $cmd -n '__fish_mkvdup_using_command mount' -l source-read-timeout -d 'Read timeout for network FS sources' -x
complete -c $cmd -n '__fish_mkvdup_using_command mount' -F -d 'Mount point or config files'

# info options
complete -c $cmd -n '__fish_mkvdup_using_command info' -l hide-unused-files -d 'Hide unused source files'
complete -c $cmd -n '__fish_mkvdup_using_command info' -F -d 'Dedup file'

# verify options
complete -c $cmd -n '__fish_mkvdup_using_command verify' -s v -l verbose -d 'Enable verbose/debug output'
complete -c $cmd -n '__fish_mkvdup_using_command verify' -s q -l quiet -d 'Suppress informational progress output'
complete -c $cmd -n '__fish_mkvdup_using_command verify' -l no-progress -d 'Disable progress bars'
complete -c $cmd -n '__fish_mkvdup_using_command verify' -F -d 'Dedup file, source directory, or original MKV'

# extract options
complete -c $cmd -n '__fish_mkvdup_using_command extract' -s v -l verbose -d 'Enable verbose/debug output'
complete -c $cmd -n '__fish_mkvdup_using_command extract' -s q -l quiet -d 'Suppress informational progress output'
complete -c $cmd -n '__fish_mkvdup_using_command extract' -l no-progress -d 'Disable progress bars'
complete -c $cmd -n '__fish_mkvdup_using_command extract' -F -d 'Dedup file, source directory, or output MKV'

# check options
complete -c $cmd -n '__fish_mkvdup_using_command check' -l source-checksums -d 'Verify source file checksums'
complete -c $cmd -n '__fish_mkvdup_using_command check' -F -d 'Dedup file or source directory'

# stats options
complete -c $cmd -n '__fish_mkvdup_using_command stats' -l config-dir -d 'Treat config argument as directory of YAML files'
complete -c $cmd -n '__fish_mkvdup_using_command stats' -F -d 'Config files'

# validate options
complete -c $cmd -n '__fish_mkvdup_using_command validate' -l config-dir -d 'Treat config argument as directory of YAML files'
complete -c $cmd -n '__fish_mkvdup_using_command validate' -l deep -d 'Verify dedup file headers and internal checksums'
complete -c $cmd -n '__fish_mkvdup_using_command validate' -l strict -d 'Treat warnings as errors'
complete -c $cmd -n '__fish_mkvdup_using_command validate' -F -d 'Config files'

# reload options
complete -c $cmd -n '__fish_mkvdup_using_command reload' -l pid-file -d 'PID file of the running daemon' -r
complete -c $cmd -n '__fish_mkvdup_using_command reload' -l pid -d 'PID of the running daemon' -r
complete -c $cmd -n '__fish_mkvdup_using_command reload' -l config-dir -d 'Treat config argument as directory of YAML files'
complete -c $cmd -n '__fish_mkvdup_using_command reload' -F -d 'Config files'

# parse-mkv options
complete -c $cmd -n '__fish_mkvdup_using_command parse-mkv' -F -d 'MKV file'

# index-source options
complete -c $cmd -n '__fish_mkvdup_using_command index-source' -F -d 'Source directory'

# match options
complete -c $cmd -n '__fish_mkvdup_using_command match' -F -d 'MKV file or source directory'

# deltadiag options
complete -c $cmd -n '__fish_mkvdup_using_command deltadiag' -s v -l verbose -d 'Enable verbose/debug output'
complete -c $cmd -n '__fish_mkvdup_using_command deltadiag' -F -d 'Dedup file or MKV file'

# help - complete with subcommand names
complete -c $cmd -n '__fish_mkvdup_using_command help' -a 'create batch-create probe mount info verify extract check stats validate reload parse-mkv index-source match deltadiag' -d 'Command'

end # for cmd
