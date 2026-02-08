# Fish completion for mkvdup
# Install to /usr/share/fish/vendor_completions.d/mkvdup.fish

# Determine the command name from the completion being registered
set -l cmd (status basename | string replace -r '\.fish$' '')
if test -z "$cmd"
    set cmd mkvdup
end

# Disable file completions by default (we'll enable them per-subcommand)
complete -c $cmd -f

# Helper function to check if a subcommand has been given
function __fish_mkvdup_needs_command
    set -l cmd (commandline -opc)
    # Skip global options to find subcommand
    for i in $cmd[2..-1]
        switch $i
            case '-v' '--verbose' '-h' '--help' '--version'
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
    set -l cmd (commandline -opc)
    set -l target $argv[1]
    for i in $cmd[2..-1]
        switch $i
            case '-v' '--verbose' '-h' '--help' '--version'
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

# Global options
complete -c $cmd -n __fish_mkvdup_needs_command -s v -l verbose -d 'Enable verbose/debug output'
complete -c $cmd -n __fish_mkvdup_needs_command -s h -l help -d 'Show help'
complete -c $cmd -n __fish_mkvdup_needs_command -l version -d 'Show version'

# Subcommands
complete -c $cmd -n __fish_mkvdup_needs_command -a create -d 'Create a dedup file from an MKV and its source directory'
complete -c $cmd -n __fish_mkvdup_needs_command -a batch-create -d 'Create multiple dedup files from a manifest'
complete -c $cmd -n __fish_mkvdup_needs_command -a probe -d 'Quick test if an MKV likely matches a source'
complete -c $cmd -n __fish_mkvdup_needs_command -a mount -d 'Mount virtual filesystem from config files'
complete -c $cmd -n __fish_mkvdup_needs_command -a info -d 'Show information about a dedup file'
complete -c $cmd -n __fish_mkvdup_needs_command -a verify -d 'Verify a dedup file against the original MKV'
complete -c $cmd -n __fish_mkvdup_needs_command -a check -d 'Check integrity of a dedup file and its source files'
complete -c $cmd -n __fish_mkvdup_needs_command -a validate -d 'Validate configuration files for correctness'
complete -c $cmd -n __fish_mkvdup_needs_command -a reload -d 'Reload a running daemon configuration'
complete -c $cmd -n __fish_mkvdup_needs_command -a parse-mkv -d 'Parse and display MKV structure (debug)'
complete -c $cmd -n __fish_mkvdup_needs_command -a index-source -d 'Index a source directory (debug)'
complete -c $cmd -n __fish_mkvdup_needs_command -a match -d 'Match packets between MKV and source (debug)'
complete -c $cmd -n __fish_mkvdup_needs_command -a deltadiag -d 'Analyze delta entries in a dedup file'
complete -c $cmd -n __fish_mkvdup_needs_command -a help -d 'Show help for a command'

# create options
complete -c $cmd -n '__fish_mkvdup_using_command create' -s v -l verbose -d 'Enable verbose/debug output'
complete -c $cmd -n '__fish_mkvdup_using_command create' -l warn-threshold -d 'Minimum space savings percentage' -x
complete -c $cmd -n '__fish_mkvdup_using_command create' -l quiet -d 'Suppress the space savings warning'
complete -c $cmd -n '__fish_mkvdup_using_command create' -l non-interactive -d 'Do not prompt on codec mismatch'
complete -c $cmd -n '__fish_mkvdup_using_command create' -F -d 'MKV file or source directory'

# batch-create options
complete -c $cmd -n '__fish_mkvdup_using_command batch-create' -s v -l verbose -d 'Enable verbose/debug output'
complete -c $cmd -n '__fish_mkvdup_using_command batch-create' -l warn-threshold -d 'Minimum space savings percentage' -x
complete -c $cmd -n '__fish_mkvdup_using_command batch-create' -l quiet -d 'Suppress the space savings warning'
complete -c $cmd -n '__fish_mkvdup_using_command batch-create' -F -d 'Manifest file'

# probe options
complete -c $cmd -n '__fish_mkvdup_using_command probe' -F -d 'MKV file or source directories'

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
complete -c $cmd -n '__fish_mkvdup_using_command mount' -F -d 'Mount point or config files'

# info options
complete -c $cmd -n '__fish_mkvdup_using_command info' -F -d 'Dedup file'

# verify options
complete -c $cmd -n '__fish_mkvdup_using_command verify' -F -d 'Dedup file, source directory, or original MKV'

# check options
complete -c $cmd -n '__fish_mkvdup_using_command check' -l source-checksums -d 'Verify source file checksums'
complete -c $cmd -n '__fish_mkvdup_using_command check' -F -d 'Dedup file or source directory'

# validate options
complete -c $cmd -n '__fish_mkvdup_using_command validate' -l config-dir -d 'Treat config argument as directory of YAML files'
complete -c $cmd -n '__fish_mkvdup_using_command validate' -l deep -d 'Verify dedup file headers and internal checksums'
complete -c $cmd -n '__fish_mkvdup_using_command validate' -l strict -d 'Treat warnings as errors'
complete -c $cmd -n '__fish_mkvdup_using_command validate' -F -d 'Config files'

# reload options
complete -c $cmd -n '__fish_mkvdup_using_command reload' -l pid-file -d 'PID file of the running daemon' -r
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
complete -c $cmd -n '__fish_mkvdup_using_command help' -a 'create batch-create probe mount info verify check validate reload parse-mkv index-source match deltadiag' -d 'Command'
