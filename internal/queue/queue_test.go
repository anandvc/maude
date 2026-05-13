package queue

import (
	"path/filepath"
	"testing"
)

func TestQueueLifecycle(t *testing.T) {
	t.Parallel()

	q, err := Open(filepath.Join(t.TempDir(), "maude.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer q.Close()

	req, err := q.Enqueue(Request{Prompt: "hello", SessionName: "default"})
	if err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}
	if req.ID == "" || req.Status != StatusQueued {
		t.Fatalf("request = %#v", req)
	}

	next, ok, err := q.NextQueued()
	if err != nil {
		t.Fatalf("NextQueued() error = %v", err)
	}
	if !ok || next.ID != req.ID {
		t.Fatalf("NextQueued() = %#v/%v", next, ok)
	}

	running, err := q.MarkRunning(req.ID)
	if err != nil {
		t.Fatalf("MarkRunning() error = %v", err)
	}
	if running.Status != StatusRunning || running.StartedAt.IsZero() {
		t.Fatalf("running = %#v", running)
	}

	done, err := q.Complete(req.ID, "answer")
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if done.Status != StatusDone || done.Output != "answer" || done.CompletedAt.IsZero() {
		t.Fatalf("done = %#v", done)
	}
}
