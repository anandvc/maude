package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dorkitude/maude/internal/config"
	"github.com/dorkitude/maude/internal/queue"
)

// TestResolveStateDirConventionIsCWDIndependent is the core regression test for
// the "request not found" divergence bug: an absolute --config that follows the
// <state_dir>/config.json convention must resolve to the SAME absolute state dir
// no matter which CWD the process runs from (enqueuer vs daemon vs the in-pane /
// Stop-hook `agent print`). Pre-migration the on-disk state_dir is still "state".
func TestResolveStateDirConventionIsCWDIndependent(t *testing.T) {
	cfg := config.Defaults() // StateDir == "state" (relative)
	abs := "/Users/anandvc/.shimmer-sessions/chatX/state/config.json"
	want := "/Users/anandvc/.shimmer-sessions/chatX/state"

	// Run the resolution from two different working directories; the convention
	// anchor must ignore CWD entirely and pick the config file's directory.
	for _, cwd := range []string{t.TempDir(), os.TempDir()} {
		dir, err := os.Getwd()
		if err != nil {
			t.Fatalf("Getwd: %v", err)
		}
		if err := os.Chdir(cwd); err != nil {
			t.Fatalf("Chdir(%s): %v", cwd, err)
		}
		got, err := resolveStateDir(cfg, abs)
		_ = os.Chdir(dir)
		if err != nil {
			t.Fatalf("resolveStateDir: %v", err)
		}
		if got.StateDir != want {
			t.Fatalf("from cwd %s: StateDir = %q, want %q", cwd, got.StateDir, want)
		}
	}
}

func TestResolveStateDirOutOfTreeFallsBackToCWD(t *testing.T) {
	cfg := config.Defaults() // StateDir "state"
	// An out-of-tree --config NOT at <state_dir>/config.json -> historical CWD anchor.
	cwd := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(cwd); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	defer os.Chdir(orig)
	got, err := resolveStateDir(cfg, "/etc/maude/custom.json")
	if err != nil {
		t.Fatalf("resolveStateDir: %v", err)
	}
	// t.TempDir() can be a symlink (/var -> /private/var on macOS); compare resolved.
	want := filepath.Join(cwd, "state")
	gotEval, _ := filepath.EvalSymlinks(got.StateDir)
	wantEval, _ := filepath.EvalSymlinks(want)
	if gotEval != wantEval {
		t.Fatalf("StateDir = %q, want %q", got.StateDir, want)
	}
}

func TestResolveStateDirAbsoluteUntouched(t *testing.T) {
	t.Parallel()
	cfg := config.Defaults()
	cfg.StateDir = "/abs/state"
	got, err := resolveStateDir(cfg, "/anywhere/state/config.json")
	if err != nil {
		t.Fatalf("resolveStateDir: %v", err)
	}
	if got.StateDir != "/abs/state" {
		t.Fatalf("absolute StateDir mutated: %q", got.StateDir)
	}
}

func TestCollectPromptFromArgs(t *testing.T) {
	t.Parallel()

	got, err := CollectPrompt([]string{"hello", "world"}, strings.NewReader(""))
	if err != nil {
		t.Fatalf("CollectPrompt() error = %v", err)
	}
	if got != "hello world" {
		t.Fatalf("prompt = %q", got)
	}
}

func TestCollectPromptCombinesArgsAndStdin(t *testing.T) {
	t.Parallel()

	got, err := CollectPrompt([]string{"review"}, strings.NewReader("diff\n"))
	if err != nil {
		t.Fatalf("CollectPrompt() error = %v", err)
	}
	if got != "review\ndiff" {
		t.Fatalf("prompt = %q", got)
	}
}

func TestRootParsesCompatibilityFlags(t *testing.T) {
	t.Parallel()

	cmd := NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"-p", "--model", "sonnet", "--permission-mode", "plan", "--no-wait", "hello"})
	if err := cmd.Flags().Parse([]string{"-p", "--model", "sonnet", "--permission-mode", "plan", "--no-wait", "hello"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	args := forwardArgs(cmd.Flags())
	want := []string{"--model", "sonnet", "--permission-mode", "plan"}
	if strings.Join(args, " ") != strings.Join(want, " ") {
		t.Fatalf("forwardArgs() = %#v, want %#v", args, want)
	}
}

func TestOutputFormatValidation(t *testing.T) {
	t.Parallel()

	cmd := NewRootCommand()
	cmd.SetArgs([]string{"-p", "--output-format", "json", "hello"})
	if err := cmd.Flags().Parse([]string{"-p", "--output-format", "json", "hello"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	got, err := outputFormat(cmd.Flags())
	if err != nil {
		t.Fatalf("outputFormat() error = %v", err)
	}
	if got != "json" {
		t.Fatalf("outputFormat() = %q, want json", got)
	}

	if err := cmd.Flags().Set("output-format", "xml"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if _, err := outputFormat(cmd.Flags()); err == nil {
		t.Fatal("outputFormat() error = nil, want invalid format error")
	}
}

func TestFormatRequestOutputJSON(t *testing.T) {
	t.Parallel()

	got, err := formatRequestOutput(queue.Request{ID: "abc", Output: "answer", OutputFormat: "json"})
	if err != nil {
		t.Fatalf("formatRequestOutput() error = %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(got), &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v; output=%s", err, got)
	}
	if payload["type"] != "result" || payload["result"] != "answer" || payload["request_id"] != "abc" {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestFormatRequestOutputStreamJSON(t *testing.T) {
	t.Parallel()

	got, err := formatRequestOutput(queue.Request{ID: "abc", Output: "answer", OutputFormat: "stream-json"})
	if err != nil {
		t.Fatalf("formatRequestOutput() error = %v", err)
	}
	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Fatalf("stream-json lines = %d, output=%s", len(lines), got)
	}
	for _, line := range lines {
		var payload map[string]any
		if err := json.Unmarshal([]byte(line), &payload); err != nil {
			t.Fatalf("Unmarshal(%q) error = %v", line, err)
		}
	}
}

func TestStreamRequestOutputEmitsChunksAsTheyArrive(t *testing.T) {
	t.Parallel()

	q, err := queue.Open(t.TempDir() + "/maude.db")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer q.Close()
	req, err := q.Enqueue(queue.Request{Prompt: "hello", OutputFormat: "stream-json", Cwd: "/tmp/project"})
	if err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}
	if _, err := q.MarkRunning(req.ID); err != nil {
		t.Fatalf("MarkRunning() error = %v", err)
	}

	go func() {
		time.Sleep(150 * time.Millisecond)
		_, _ = q.AppendOutput(req.ID, "alpha")
		time.Sleep(150 * time.Millisecond)
		_, _ = q.AppendOutput(req.ID, " beta")
		time.Sleep(150 * time.Millisecond)
		_, _ = q.Complete(req.ID, "alpha beta")
	}()

	cfg := config.Defaults()
	cfg.RequestTimeout = "2s"
	var out bytes.Buffer
	if err := streamRequestOutput(&out, q, req.ID, cfg); err != nil {
		t.Fatalf("streamRequestOutput() error = %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 4 {
		t.Fatalf("got %d stream lines, want 4:\n%s", len(lines), out.String())
	}
	var first map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("Unmarshal(init) error = %v", err)
	}
	if first["type"] != "system" || first["subtype"] != "init" {
		t.Fatalf("init event = %#v", first)
	}
	if !strings.Contains(out.String(), "alpha") || !strings.Contains(out.String(), " beta") {
		t.Fatalf("stream output missing chunks:\n%s", out.String())
	}
}

func TestStreamAgentPrintAppendsOutputBeforeComplete(t *testing.T) {
	t.Parallel()

	q, err := queue.Open(t.TempDir() + "/maude.db")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer q.Close()
	req, err := q.Enqueue(queue.Request{Prompt: "hello", OutputFormat: "stream-json"})
	if err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}
	if _, err := q.MarkRunning(req.ID); err != nil {
		t.Fatalf("MarkRunning() error = %v", err)
	}
	if err := streamAgentPrint(strings.NewReader("hello\n"), q, req.ID); err != nil {
		t.Fatalf("streamAgentPrint() error = %v", err)
	}
	got, err := q.Get(req.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Status != queue.StatusDone || got.Output != "hello" {
		t.Fatalf("request = %#v", got)
	}
}

// Regression (PR #2 review): `daemon run` must start with a read-only config
// whose state_dir is already absolute; it must not attempt a write at all.
func TestPersistResolvedStateDirSkipsUnchangedAndToleratesReadOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	cfg := config.Defaults()
	cfg.StateDir = filepath.Join(dir, "state")
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0o755)
	before, _ := os.ReadFile(path)

	var warn bytes.Buffer
	if persistResolvedStateDir(&warn, path, cfg, cfg) {
		t.Fatal("unchanged absolute state_dir must not be rewritten")
	}
	if warn.Len() != 0 {
		t.Fatalf("unexpected warning: %q", warn.String())
	}

	// A relative on-disk value that needs migrating, but the dir is read-only:
	// warn, do not fail, and leave the file untouched.
	raw := cfg
	raw.StateDir = "state"
	if persistResolvedStateDir(&warn, path, raw, cfg) {
		t.Fatal("write into a read-only dir must not report success")
	}
	if !strings.Contains(warn.String(), "could not persist") {
		t.Fatalf("expected a warning, got %q", warn.String())
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("config changed despite failed write")
	}
}
