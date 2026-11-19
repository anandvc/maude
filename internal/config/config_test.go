package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := Load(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	want := Defaults()
	if !reflect.DeepEqual(cfg, want) {
		t.Fatalf("Load() = %#v, want %#v", cfg, want)
	}
}

func TestSaveAndLoadPersistsJSON(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "nested", "config.json")
	cfg := Defaults()
	cfg.StateDir = "/tmp/maude-state"
	cfg.DefaultSession = "work"
	cfg.TmuxPrefix = "custom"
	cfg.ClaudeBinary = "/opt/bin/claude"
	cfg.StartupWait = "3s"
	cfg.WaitTimeout = "45s"
	cfg.WaitPollInterval = "750ms"
	cfg.CaptureDelay = "100ms"
	cfg.CaptureLines = 75
	cfg.DaemonPoll = "200ms"
	cfg.RequestTimeout = "2m"

	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(data), `"state_dir": "/tmp/maude-state"`) {
		t.Fatalf("saved config did not contain expected state_dir: %s", data)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !reflect.DeepEqual(got, cfg) {
		t.Fatalf("Load() = %#v, want %#v", got, cfg)
	}
}

func TestResolveStateDir(t *testing.T) {
	t.Parallel()

	// Relative StateDir is anchored to baseDir.
	cfg := Defaults() // StateDir == "state"
	got := cfg.ResolveStateDir("/base/chat")
	if want := filepath.Join("/base/chat", "state"); got.StateDir != want {
		t.Fatalf("relative ResolveStateDir = %q, want %q", got.StateDir, want)
	}

	// Absolute StateDir is left untouched.
	cfg = Defaults()
	cfg.StateDir = "/abs/state"
	if got := cfg.ResolveStateDir("/base/chat"); got.StateDir != "/abs/state" {
		t.Fatalf("absolute ResolveStateDir = %q, want /abs/state", got.StateDir)
	}

	// Empty baseDir leaves the value untouched (no CWD lookup).
	cfg = Defaults()
	if got := cfg.ResolveStateDir(""); got.StateDir != "state" {
		t.Fatalf("empty baseDir ResolveStateDir = %q, want state", got.StateDir)
	}

	// Empty StateDir falls back to the default name under baseDir.
	cfg = Defaults()
	cfg.StateDir = ""
	if got := cfg.ResolveStateDir("/base/chat"); got.StateDir != filepath.Join("/base/chat", "state") {
		t.Fatalf("empty StateDir ResolveStateDir = %q, want /base/chat/state", got.StateDir)
	}

	// Idempotent: resolving an already-resolved config does not re-anchor.
	cfg = Defaults().ResolveStateDir("/base/chat")
	if again := cfg.ResolveStateDir("/other/cwd"); again.StateDir != filepath.Join("/base/chat", "state") {
		t.Fatalf("ResolveStateDir not idempotent: %q", again.StateDir)
	}
}

func TestDurationHelpers(t *testing.T) {
	t.Parallel()

	cfg := Defaults()
	got, err := cfg.StartupWaitDuration()
	if err != nil {
		t.Fatalf("StartupWaitDuration() error = %v", err)
	}
	if got != 2*time.Second {
		t.Fatalf("StartupWaitDuration() = %v, want 2s", got)
	}

	cfg.WaitTimeout = "nope"
	if _, err := cfg.WaitTimeoutDuration(); err == nil {
		t.Fatal("WaitTimeoutDuration() error = nil, want parse error")
	}

	cfg = Defaults()
	if got, err := cfg.RequestTimeoutDuration(); err != nil || got != 10*time.Minute {
		t.Fatalf("RequestTimeoutDuration() = %v/%v, want 10m/nil", got, err)
	}
}

func TestDefaultPath(t *testing.T) {
	t.Parallel()

	if got := DefaultPath(); got != filepath.Join("state", "config.json") {
		t.Fatalf("DefaultPath() = %q", got)
	}
}
