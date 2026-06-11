package credential

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// DefaultPath is the on-disk location of the active session.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".istorepull", "session.json"), nil
}

// Save writes the session to path with 0600 permissions (it holds a live token).
func Save(path string, s Session) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create session dir: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write session: %w", err)
	}
	return nil
}

// Load reads a session from path.
func Load(path string) (Session, error) {
	var s Session
	data, err := os.ReadFile(path)
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return s, fmt.Errorf("parse session %s: %w", path, err)
	}
	return s, nil
}
