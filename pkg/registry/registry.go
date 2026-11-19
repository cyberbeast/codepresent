// Package registry manages file-based session discovery for running codepresent instances.
package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

// Session describes an active codepresent server instance registered for discovery.
type Session struct {
	PID           int       `json:"pid"`
	WSURL         string    `json:"ws_url"`
	WorkspaceRoot string    `json:"workspace_root,omitempty"`
	StoryTitle    string    `json:"story_title,omitempty"`
	StartedAt     time.Time `json:"started_at"`
}

// DefaultDir returns the default session registry directory (~/.codepresent/sessions).
func DefaultDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home dir: %w", err)
	}
	return filepath.Join(home, ".codepresent", "sessions"), nil
}

// Write atomically writes a session entry to dir/<pid>.json.
func Write(dir string, s Session) error {
	if s.PID <= 0 {
		return fmt.Errorf("invalid session pid: %d", s.PID)
	}
	if s.WSURL == "" {
		return errors.New("session ws_url is required")
	}
	if s.StartedAt.IsZero() {
		s.StartedAt = time.Now().UTC()
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create registry dir: %w", err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	target := filepath.Join(dir, fmt.Sprintf("%d.json", s.PID))
	tmp := filepath.Join(dir, fmt.Sprintf(".%d.json.tmp", s.PID))
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write temp session file: %w", err)
	}
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename session file: %w", err)
	}
	return nil
}

// Remove deletes the session file for pid from dir, ignoring missing files.
func Remove(dir string, pid int) error {
	target := filepath.Join(dir, fmt.Sprintf("%d.json", pid))
	if err := os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// List returns active sessions in dir sorted newest-first, pruning entries whose PID is no longer alive.
func List(dir string) ([]Session, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read registry dir: %w", err)
	}

	var sessions []Session
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		fullPath := filepath.Join(dir, entry.Name())
		raw, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}
		var s Session
		if err := json.Unmarshal(raw, &s); err != nil {
			continue
		}
		if s.PID <= 0 || !IsProcessAlive(s.PID) {
			_ = os.Remove(fullPath)
			continue
		}
		sessions = append(sessions, s)
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].StartedAt.After(sessions[j].StartedAt)
	})
	return sessions, nil
}

// SelectForWorkspace returns the best matching active session for workspace,
// falling back to the sole active session if only one exists.
func SelectForWorkspace(sessions []Session, workspace string) (Session, bool) {
	if len(sessions) == 0 {
		return Session{}, false
	}

	cleanTarget := cleanPath(workspace)
	if cleanTarget != "" {
		for _, s := range sessions {
			cleanRoot := cleanPath(s.WorkspaceRoot)
			if cleanRoot == "" {
				continue
			}
			if cleanTarget == cleanRoot || isSubpath(cleanTarget, cleanRoot) || isSubpath(cleanRoot, cleanTarget) {
				return s, true
			}
		}
	}

	if len(sessions) == 1 {
		return sessions[0], true
	}
	return Session{}, false
}

// IsProcessAlive reports whether a process with pid exists on the local host.
func IsProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

func cleanPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	return filepath.Clean(p)
}

func isSubpath(child, parent string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
