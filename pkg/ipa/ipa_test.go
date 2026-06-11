package ipa

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"howett.net/plist"
)

// makeSrcZip writes a minimal App Store-style zip and returns its path.
func makeSrcZip(t *testing.T, withManifest bool) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "src.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	zw := zip.NewWriter(f)
	app := "Payload/Demo.app/"

	info, _ := plist.MarshalIndent(map[string]any{"CFBundleExecutable": "Demo"}, plist.XMLFormat, "  ")
	mustWrite(t, zw, app+"Info.plist", info)
	mustWrite(t, zw, app+"Demo", []byte("MACHO"))

	if withManifest {
		man, _ := plist.MarshalIndent(map[string]any{
			"SinfPaths": []string{"SC_Info/Demo.sinf"},
		}, plist.XMLFormat, "  ")
		mustWrite(t, zw, app+"SC_Info/Manifest.plist", man)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func mustWrite(t *testing.T, zw *zip.Writer, name string, data []byte) {
	t.Helper()
	w, err := zw.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
}

func readEntry(t *testing.T, zipPath, name string) ([]byte, bool) {
	t.Helper()
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = zr.Close() }()
	for _, f := range zr.File {
		if f.Name == name {
			rc, _ := f.Open()
			defer func() { _ = rc.Close() }()
			b, _ := io.ReadAll(rc)
			return b, true
		}
	}
	return nil, false
}

func TestBuildWithManifest(t *testing.T) {
	src := makeSrcZip(t, true)
	dst := filepath.Join(t.TempDir(), "out.ipa")

	err := Build(BuildInput{
		SrcZip:   src,
		DstPath:  dst,
		Sinfs:    []Sinf{{ID: 0, Data: []byte("SINFDATA")}},
		Metadata: map[string]any{"bundleShortVersionString": "17.4.0"},
		AppleID:  "user@example.com",
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	sinf, ok := readEntry(t, dst, "Payload/Demo.app/SC_Info/Demo.sinf")
	if !ok || !bytes.Equal(sinf, []byte("SINFDATA")) {
		t.Errorf("sinf at manifest path missing/incorrect: %q ok=%v", sinf, ok)
	}

	meta, ok := readEntry(t, dst, "iTunesMetadata.plist")
	if !ok {
		t.Fatal("iTunesMetadata.plist missing")
	}
	var m map[string]any
	if _, err := plist.Unmarshal(meta, &m); err != nil {
		t.Fatal(err)
	}
	if m["apple-id"] != "user@example.com" {
		t.Errorf("apple-id not embedded: %v", m["apple-id"])
	}
	if m["bundleShortVersionString"] != "17.4.0" {
		t.Errorf("metadata not preserved: %v", m)
	}
}

func TestBuildFallbackSinfPath(t *testing.T) {
	src := makeSrcZip(t, false) // no Manifest -> SC_Info/<exe>.sinf
	dst := filepath.Join(t.TempDir(), "out.ipa")

	err := Build(BuildInput{
		SrcZip:  src,
		DstPath: dst,
		Sinfs:   []Sinf{{ID: 0, Data: []byte("X")}},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if _, ok := readEntry(t, dst, "Payload/Demo.app/SC_Info/Demo.sinf"); !ok {
		t.Error("fallback sinf path not written")
	}
}

func TestBuildSinfCountMismatch(t *testing.T) {
	src := makeSrcZip(t, true)
	dst := filepath.Join(t.TempDir(), "out.ipa")
	err := Build(BuildInput{
		SrcZip:  src,
		DstPath: dst,
		Sinfs:   nil, // 1 path, 0 sinfs
	})
	if err == nil {
		t.Fatal("expected mismatch error")
	}
}
