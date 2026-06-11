package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pixel0verflow/istorepull/pkg/itunes"
)

// resolveAdamID returns the adam id from an explicit id, or by looking up a
// bundle id against the public iTunes API.
func resolveAdamID(ctx context.Context, bundleID string, adamID int64, country string) (int64, error) {
	switch {
	case adamID != 0 && bundleID != "":
		return 0, fmt.Errorf("%w: specify only one of -b/--bundle or -i/--id", errBadInput)
	case adamID != 0:
		return adamID, nil
	case bundleID != "":
		app, err := itunes.New().LookupBundle(ctx, bundleID, country)
		if err != nil {
			return 0, fmt.Errorf("resolve bundle %s: %w", bundleID, err)
		}
		if app.ID == 0 {
			return 0, fmt.Errorf("%w: no adam id for bundle %s", errBadInput, bundleID)
		}
		return app.ID, nil
	default:
		return 0, fmt.Errorf("%w: specify -b/--bundle or -i/--id", errBadInput)
	}
}

// vermapPath returns the on-disk cache for an adam id's id->version map.
func vermapPath(adamID int64) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".istorepull", "vermap", fmt.Sprintf("%d.json", adamID)), nil
}

// loadVermap reads the cached id->version map (empty on miss).
func loadVermap(adamID int64) map[string]string {
	path, err := vermapPath(adamID)
	if err != nil {
		return map[string]string{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]string{}
	}
	m := map[string]string{}
	_ = json.Unmarshal(data, &m)
	return m
}

// saveVermap persists the id->version map (best effort).
func saveVermap(adamID int64, m map[string]string) {
	path, err := vermapPath(adamID)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o600)
}
