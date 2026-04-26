module github.com/stuckj/mkvdup

go 1.25.0

toolchain go1.25.8

require (
	github.com/cespare/xxhash/v2 v2.3.0
	github.com/hanwen/go-fuse/v2 v2.10.1
	golang.org/x/sys v0.42.0
)

require gopkg.in/yaml.v3 v3.0.1

require github.com/bmatcuk/doublestar/v4 v4.10.0

require github.com/fsnotify/fsnotify v1.9.0

require al.essio.dev/pkg/shellescape v1.6.0

require (
	github.com/bitfield/gotestdox v0.2.2 // indirect
	github.com/dnephin/pflag v1.0.7 // indirect
	github.com/fatih/color v1.18.0 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/wadey/gocovmerge v0.0.0-20160331181800-b5bfa59ec0ad // indirect
	golang.org/x/mod v0.27.0 // indirect
	golang.org/x/sync v0.17.0 // indirect
	golang.org/x/term v0.35.0 // indirect
	golang.org/x/text v0.17.0 // indirect
	golang.org/x/tools v0.36.0 // indirect
	gotest.tools/gotestsum v1.13.0 // indirect
)

tool (
	github.com/wadey/gocovmerge
	gotest.tools/gotestsum
)
