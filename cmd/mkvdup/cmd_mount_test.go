package main

import (
	"os"
	"os/signal"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestReloadDaemon_DeadProcess(t *testing.T) {
	// Use a very high PID that almost certainly doesn't exist
	err := reloadDaemon(4194304, nil, false)
	if err == nil {
		t.Error("expected error for non-running process")
	}
	if !strings.Contains(err.Error(), "not running") {
		t.Errorf("expected 'not running' error, got: %v", err)
	}
}

func TestReloadDaemon_SendsSignal(t *testing.T) {
	// Ignore SIGHUP so we don't die when the function sends it to us.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGHUP)
	defer signal.Stop(sigCh)

	err := reloadDaemon(os.Getpid(), nil, false)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Drain the signal we sent to ourselves
	select {
	case <-sigCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for SIGHUP signal")
	}
}
