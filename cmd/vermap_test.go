package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVerCacheRoundTripAndLookup(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := loadVerCache(357218860)
	if len(c.m) != 0 {
		t.Fatalf("fresh cache should be empty, got %v", c.m)
	}
	c.put("886534247", "17.13.2")
	c.put("878571262", "17.8.1")
	c.put("886534247", "17.13.2") // duplicate, no-op
	c.save()

	got := loadVerCache(357218860)
	if v, ok := got.version("878571262"); !ok || v != "17.8.1" {
		t.Errorf("version lookup = %q,%v", v, ok)
	}
	if id, ok := got.findByVersion("17.13.2"); !ok || id != "886534247" {
		t.Errorf("findByVersion = %q,%v", id, ok)
	}
	if _, ok := got.findByVersion("1.2.3"); ok {
		t.Error("findByVersion should miss for unknown version")
	}
}

func TestVerCacheFindByVersionPrefersNewest(t *testing.T) {
	c := verCache{adamID: 1, m: map[string]string{
		"885093713": "17.12.1",
		"885256351": "17.12.1", // newer build, same version string
	}}
	id, ok := c.findByVersion("17.12.1")
	if !ok || id != "885256351" {
		t.Errorf("findByVersion = %q,%v, want newest 885256351", id, ok)
	}
}

func TestVerCacheSaveSkippedWhenClean(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	c := loadVerCache(42)
	c.save() // not dirty → must not create a file
	path, _ := vermapPath(42)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("clean cache should not write a file (err=%v)", err)
	}
}

func TestClearVerCache(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	c := loadVerCache(99)
	c.put("1", "1.0")
	c.save()
	path, _ := vermapPath(99)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected cache file: %v", err)
	}
	if err := clearVerCache(99); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("cache file should be gone after clear")
	}
	// clearing a missing cache is not an error
	if err := clearVerCache(12345); err != nil {
		t.Errorf("clear missing = %v", err)
	}
}

func TestVermapPathLocation(t *testing.T) {
	t.Setenv("HOME", "/tmp/fakehome")
	path, err := vermapPath(7)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join("/tmp/fakehome", ".istorepull", "vermap", "7.json")
	if path != want {
		t.Errorf("path = %q, want %q", path, want)
	}
}
