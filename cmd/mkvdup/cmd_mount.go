package main

import (
	"fmt"
	"log"
	"log/syslog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/stuckj/mkvdup/internal/daemon"
	"github.com/stuckj/mkvdup/internal/dedup"
	mkvfuse "github.com/stuckj/mkvdup/internal/fuse"
)

// defaultConfigPath is the default config file location.
const defaultConfigPath = "/etc/mkvdup.conf"

// expandConfigDir expands a directory path to a list of .yaml/.yml files it contains.
func expandConfigDir(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read config directory %s: %w", dir, err)
	}
	var paths []string
	for _, entry := range entries {
		if !entry.IsDir() && (filepath.Ext(entry.Name()) == ".yaml" || filepath.Ext(entry.Name()) == ".yml") {
			paths = append(paths, filepath.Join(dir, entry.Name()))
		}
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no YAML files (.yaml, .yml) found in %s", dir)
	}
	return paths, nil
}

// mountFuse mounts a FUSE filesystem exposing dedup files as MKV files.
func mountFuse(mountpoint string, configPaths []string, opts MountOptions) error {
	// Daemonize unless --foreground is set or we're already a daemon child
	if !opts.Foreground && !daemon.IsChild() {
		return daemon.Daemonize(opts.PidFile, opts.DaemonTimeout)
	}

	// Write PID file in foreground mode (daemon mode writes it in Daemonize)
	if opts.Foreground && opts.PidFile != "" {
		if err := daemon.WritePidFile(opts.PidFile, os.Getpid()); err != nil {
			return fmt.Errorf("write pid file: %w", err)
		}
	}

	// Clean up PID file on exit (for both foreground and daemon child modes)
	if opts.PidFile != "" && (opts.Foreground || daemon.IsChild()) {
		defer func() {
			_ = daemon.RemovePidFile(opts.PidFile)
		}()
	}

	// If no config paths provided, use default
	if len(configPaths) == 0 {
		if _, err := os.Stat(defaultConfigPath); err == nil {
			configPaths = []string{defaultConfigPath}
		} else {
			if daemon.IsChild() {
				daemon.NotifyError(fmt.Errorf("no config files specified and %s not found", defaultConfigPath))
			}
			return fmt.Errorf("no config files specified and %s not found", defaultConfigPath)
		}
	}

	// Store the config-dir path for SIGHUP re-expansion
	var configDirPath string
	if opts.ConfigDir {
		configDirPath = configPaths[0]
	}

	// If configDir is set, expand directory to list of .yaml files
	if opts.ConfigDir {
		if len(configPaths) != 1 {
			err := fmt.Errorf("--config-dir requires exactly one directory path, got %d", len(configPaths))
			if daemon.IsChild() {
				daemon.NotifyError(err)
			}
			return err
		}
		expanded, err := expandConfigDir(configPaths[0])
		if err != nil {
			if daemon.IsChild() {
				daemon.NotifyError(err)
			}
			return err
		}
		configPaths = expanded
	}

	// Set up permission store
	defaults := mkvfuse.Defaults{
		FileUID:  opts.DefaultUID,
		FileGID:  opts.DefaultGID,
		FileMode: opts.DefaultFileMode,
		DirUID:   opts.DefaultUID,
		DirGID:   opts.DefaultGID,
		DirMode:  opts.DefaultDirMode,
	}
	permPath := mkvfuse.ResolvePermissionsPath(opts.PermissionsFile)
	permStore := mkvfuse.NewPermissionStore(permPath, defaults, verbose)
	if err := permStore.Load(); err != nil {
		if daemon.IsChild() {
			daemon.NotifyError(fmt.Errorf("load permissions: %w", err))
		}
		return fmt.Errorf("load permissions: %w", err)
	}

	// Resolve configs (expands includes, globs, virtual_files) and extract
	// on_error_command (first-wins across all config files).
	configs, errorCmdConfig, err := dedup.ResolveConfigs(configPaths)
	if err != nil {
		err = fmt.Errorf("resolve configs: %w", err)
		if daemon.IsChild() {
			daemon.NotifyError(err)
		}
		return err
	}
	opts.OnErrorCommand = errorCmdConfig

	// Create the root filesystem
	root, err := mkvfuse.NewMKVFSFromConfigs(configs, verbose, &mkvfuse.DefaultReaderFactory{ReadTimeout: opts.SourceReadTimeout}, permStore)
	if err != nil {
		err = fmt.Errorf("create filesystem: %w", err)
		if daemon.IsChild() {
			daemon.NotifyError(err)
		}
		return err
	}

	// Mount the filesystem
	fuseOpts := &fs.Options{
		MountOptions: fuse.MountOptions{
			AllowOther: opts.AllowOther,
			Name:       "mkvdup",
			FsName:     "mkvdup",
			MaxWrite:   1 << 20, // 1MB max read/write; go-fuse sets max_read = MaxWrite
			// Enable kernel permission checks for standard Unix semantics.
			// This properly handles supplementary groups and matches behavior
			// of real filesystems (ext4, XFS, btrfs, etc.).
			Options: []string{"default_permissions"},
		},
	}

	server, err := fs.Mount(mountpoint, root, fuseOpts)
	if err != nil {
		err = fmt.Errorf("mount: %w", err)
		if daemon.IsChild() {
			daemon.NotifyError(err)
		}
		return err
	}

	// Wait for mount to be ready
	server.WaitMount()

	// Enable FUSE kernel notifications (NotifyDelete, NotifyEntry, etc.)
	// now that the go-fuse bridge is initialized.
	root.SetMounted()

	// Set up source file watcher (monitors source files for changes)
	var sourceWatcher *mkvfuse.SourceWatcher
	if !opts.NoSourceWatch {
		// Closure over log.Printf: syslog setup below redirects the default
		// logger's output, so the watcher automatically picks it up.
		watchLogFn := func(format string, args ...interface{}) {
			log.Printf(format, args...)
		}
		var err error
		sourceWatcher, err = mkvfuse.NewSourceWatcher(opts.OnSourceChange, opts.SourceWatchPollInterval, opts.OnErrorCommand, watchLogFn)
		if err != nil {
			log.Printf("source-watch: warning: failed to create watcher: %v", err)
		} else {
			sourceWatcher.Update(root.Files(), &mkvfuse.DefaultReaderFactory{ReadTimeout: opts.SourceReadTimeout})
			sourceWatcher.Start()
		}
	}

	// If we're a daemon child, signal success and detach from terminal
	if daemon.IsChild() {
		if err := daemon.NotifyReady(); err != nil {
			// Parent may have timed out; log and continue since mount succeeded
			fmt.Fprintf(os.Stderr, "warning: failed to notify parent: %v\n", err)
		}
		daemon.Detach()
	} else {
		// Running in foreground mode - print info
		fmt.Printf("Mounted at %s\n", mountpoint)
		fmt.Printf("Files:\n")
		for _, configPath := range configPaths {
			config, _ := dedup.ReadConfig(configPath)
			if config != nil {
				fmt.Printf("  %s\n", config.Name)
			}
		}
		fmt.Println()
		fmt.Println("Press Ctrl+C to unmount")
	}

	// Set up logging function. In daemon mode, use syslog since
	// stderr is redirected to /dev/null after Detach().
	logFn := func(format string, args ...interface{}) {
		log.Printf(format, args...)
	}
	var syslogWriter *syslog.Writer
	if daemon.IsChild() {
		if w, err := syslog.New(syslog.LOG_INFO|syslog.LOG_DAEMON, "mkvdup"); err == nil {
			syslogWriter = w
			logFn = func(format string, args ...interface{}) {
				syslogWriter.Info(fmt.Sprintf(format, args...))
			}
		}
	}
	if syslogWriter != nil {
		// Redirect global log output to syslog so that log.Printf calls
		// from BuildDirectoryTree (during reload) go to syslog too.
		log.SetOutput(syslogWriter)
		log.SetFlags(0) // syslog adds its own timestamp
		defer syslogWriter.Close()
	}

	// Handle signals for graceful shutdown and config reload
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	go func() {
		for sig := range sigChan {
			switch sig {
			case syscall.SIGHUP:
				logFn("received SIGHUP, reloading config...")

				// Re-expand config-dir if applicable
				var reloadPaths []string
				if configDirPath != "" {
					expanded, err := expandConfigDir(configDirPath)
					if err != nil {
						logFn("reload failed: expand config dir: %v", err)
						continue
					}
					reloadPaths = expanded
				} else {
					reloadPaths = configPaths
				}

				// Resolve configs (expands includes, globs, virtual_files)
				configs, _, err := dedup.ResolveConfigs(reloadPaths)
				if err != nil {
					logFn("reload failed: resolve configs: %v", err)
					continue
				}

				// Reload the filesystem
				if err := root.Reload(configs, logFn); err != nil {
					logFn("reload failed: %v", err)
					continue
				}

				// Update source watcher with new file set
				if sourceWatcher != nil {
					sourceWatcher.Update(root.Files(), &mkvfuse.DefaultReaderFactory{ReadTimeout: opts.SourceReadTimeout})
				}

				logFn("config reloaded successfully")

			case syscall.SIGINT, syscall.SIGTERM:
				if !daemon.IsChild() {
					fmt.Println("\nUnmounting...")
				}
				server.Unmount()
				return
			}
		}
	}()

	// Serve until unmounted
	server.Wait()

	// Stop source watcher
	if sourceWatcher != nil {
		sourceWatcher.Stop()
	}

	if !daemon.IsChild() {
		fmt.Println("Unmounted")
	}

	return nil
}

// reloadDaemon validates config files and sends SIGHUP to the running daemon.
func reloadDaemon(pid int, configPaths []string, configDir bool) error {
	// Verify the process exists (on Unix, FindProcess always succeeds;
	// send signal 0 to check if process is actually running)
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process %d: %w", pid, err)
	}
	if err := process.Signal(syscall.Signal(0)); err != nil {
		return fmt.Errorf("daemon process %d is not running: %w", pid, err)
	}

	// Validate config if paths provided
	if len(configPaths) > 0 {
		resolved, err := resolveConfigPaths(configPaths, configDir)
		if err != nil {
			return fmt.Errorf("resolve config paths: %w", err)
		}

		fmt.Println("Validating configuration...")
		allEntries, _, hasErrors := validateConfigEntries(resolved)
		nameErrors, _ := checkNameConflicts(allEntries)
		if hasErrors || nameErrors {
			return fmt.Errorf("config validation failed, not sending reload signal")
		}
		fmt.Println("Configuration valid.")
		fmt.Println()
	}

	// Send SIGHUP to the daemon
	fmt.Printf("Sending SIGHUP to daemon (pid %d)...\n", pid)
	if err := process.Signal(syscall.SIGHUP); err != nil {
		return fmt.Errorf("send SIGHUP to process %d: %w", pid, err)
	}

	fmt.Println("Reload signal sent successfully.")
	return nil
}

// resolveConfigPaths expands --config-dir and applies defaults to get the final
// list of config file paths to validate.
func resolveConfigPaths(configPaths []string, configDir bool) ([]string, error) {
	if configDir {
		if len(configPaths) != 1 {
			return nil, fmt.Errorf("--config-dir requires exactly one directory path, got %d", len(configPaths))
		}
		return expandConfigDir(configPaths[0])
	}

	if len(configPaths) == 0 {
		return nil, fmt.Errorf("no config files specified\nRun 'mkvdup validate --help' for usage")
	}

	return configPaths, nil
}
