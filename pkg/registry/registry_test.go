package registry

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteListAndRemove(t *testing.T) {
	dir := t.TempDir()
	pid := os.Getpid()

	first := Session{
		PID:           pid,
		WSURL:         "ws://127.0.0.1:50001/ws",
		WorkspaceRoot: "/tmp/repo-a",
		StoryTitle:    "First Story",
		StartedAt:     time.Now().UTC().Add(-time.Minute),
	}
	if err := Write(dir, first); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Stale PID should be pruned automatically by List.
	stalePath := filepath.Join(dir, "999999.json")
	if err := os.WriteFile(stalePath, []byte(`{"pid":999999,"ws_url":"ws://127.0.0.1:59999/ws"}`), 0o644); err != nil {
		t.Fatalf("write stale file failed: %v", err)
	}

	sessions, err := List(dir)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 active session, got %d", len(sessions))
	}
	if sessions[0].WSURL != first.WSURL || sessions[0].StoryTitle != first.StoryTitle {
		t.Fatalf("unexpected session: %+v", sessions[0])
	}
	if _, err := os.Stat(stalePath); !os.IsNotExist(err) {
		t.Fatalf("expected stale session file to be pruned")
	}

	if err := Remove(dir, pid); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}
	sessions, err = List(dir)
	if err != nil {
		t.Fatalf("List after Remove failed: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions after Remove, got %d", len(sessions))
	}
}

func TestSelectForWorkspace(t *testing.T) {
	sessions := []Session{
		{PID: 1, WSURL: "ws://127.0.0.1:50002/ws", WorkspaceRoot: "/Users/dev/repo-b"},
		{PID: 2, WSURL: "ws://127.0.0.1:50001/ws", WorkspaceRoot: "/Users/dev/repo-a"},
	}

	got, ok := SelectForWorkspace(sessions, "/Users/dev/repo-a/pkg")
	if !ok || got.PID != 2 {
		t.Fatalf("expected PID 2 for subpath match, got ok=%v session=%+v", ok, got)
	}

	if _, ok := SelectForWorkspace(sessions, "/Users/dev/other"); ok {
		t.Fatalf("expected no match when multiple unrelated sessions exist")
	}

	single := []Session{sessions[0]}
	got, ok = SelectForWorkspace(single, "/Users/dev/other")
	if !ok || got.PID != 1 {
		t.Fatalf("expected fallback to sole active session, got ok=%v session=%+v", ok, got)
	}
}
