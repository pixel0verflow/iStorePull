package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// verCache is a per-app cache mapping external version id -> version string,
// persisted at ~/.istorepull/vermap/<adamId>.json. ids and their versions are
// immutable, so the cache never needs invalidation (only an explicit clear).
type verCache struct {
	adamID int64
	m      map[string]string
	dirty  bool
}

func vermapPath(adamID int64) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".istorepull", "vermap", fmt.Sprintf("%d.json", adamID)), nil
}

// loadVerCache reads the cache for an adam id (empty on miss).
func loadVerCache(adamID int64) verCache {
	c := verCache{adamID: adamID, m: map[string]string{}}
	path, err := vermapPath(adamID)
	if err != nil {
		return c
	}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &c.m)
	}
	return c
}

// version returns the cached version string for an external id.
func (c verCache) version(extID string) (string, bool) {
	v, ok := c.m[extID]
	return v, ok
}

// findByVersion returns the newest cached external id whose version matches
// (newest = largest numeric id), mirroring the live newest-first resolution.
func (c verCache) findByVersion(version string) (string, bool) {
	best := ""
	for id, v := range c.m {
		if v != version {
			continue
		}
		if best == "" || newerID(id, best) {
			best = id
		}
	}
	return best, best != ""
}

// newerID reports whether external id a is newer (numerically larger) than b.
func newerID(a, b string) bool {
	ai, errA := strconv.ParseInt(a, 10, 64)
	bi, errB := strconv.ParseInt(b, 10, 64)
	if errA == nil && errB == nil {
		return ai > bi
	}
	return a > b
}

// put records an id->version mapping.
func (c *verCache) put(extID, version string) {
	if version == "" {
		return
	}
	if c.m[extID] != version {
		c.m[extID] = version
		c.dirty = true
	}
}

// save persists the cache if it changed (best effort).
func (c verCache) save() {
	if !c.dirty {
		return
	}
	path, err := vermapPath(c.adamID)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	data, err := json.MarshalIndent(c.m, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o600)
}

// clearVerCache deletes the cached map for an adam id.
func clearVerCache(adamID int64) error {
	path, err := vermapPath(adamID)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
