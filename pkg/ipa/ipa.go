// Package ipa assembles an installable IPA from a downloaded App Store zip by
// injecting the FairPlay sinf blobs and an iTunesMetadata.plist, exactly as
// Apple's own clients do. It never decrypts anything.
package ipa

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"howett.net/plist"
)

// Sinf is a FairPlay supplemental info blob keyed by its id.
type Sinf struct {
	ID   int64
	Data []byte
}

// BuildInput holds everything needed to assemble the final IPA.
type BuildInput struct {
	SrcZip   string         // path to the downloaded App Store zip
	DstPath  string         // path to write the finished IPA
	Sinfs    []Sinf         // FairPlay sinfs from the download ticket
	Metadata map[string]any // item metadata from the download ticket
	AppleID  string         // account email (optional, embedded in metadata)
}

// Build reads SrcZip, injects sinfs + iTunesMetadata.plist, and writes DstPath.
func Build(in BuildInput) error {
	zr, err := zip.OpenReader(in.SrcZip)
	if err != nil {
		return fmt.Errorf("open downloaded zip: %w", err)
	}
	defer func() { _ = zr.Close() }()

	appDir, err := findAppDir(&zr.Reader)
	if err != nil {
		return err
	}
	sinfPaths, err := resolveSinfPaths(&zr.Reader, appDir)
	if err != nil {
		return err
	}
	if len(sinfPaths) != len(in.Sinfs) {
		return fmt.Errorf("sinf count mismatch: %d paths, %d sinfs", len(sinfPaths), len(in.Sinfs))
	}

	out, err := os.Create(in.DstPath)
	if err != nil {
		return fmt.Errorf("create output: %w", err)
	}
	defer func() { _ = out.Close() }()

	zw := zip.NewWriter(out)
	if err := copyEntries(zw, &zr.Reader); err != nil {
		return err
	}
	if err := writeSinfs(zw, sinfPaths, in.Sinfs); err != nil {
		return err
	}
	if err := writeMetadata(zw, in.Metadata, in.AppleID); err != nil {
		return err
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("finalize ipa: %w", err)
	}
	return nil
}

// findAppDir returns the "Payload/<App>.app" prefix (with trailing slash).
func findAppDir(zr *zip.Reader) (string, error) {
	for _, f := range zr.File {
		if !strings.HasPrefix(f.Name, "Payload/") {
			continue
		}
		rest := strings.TrimPrefix(f.Name, "Payload/")
		i := strings.Index(rest, ".app/")
		if i < 0 {
			continue
		}
		return "Payload/" + rest[:i+len(".app")] + "/", nil
	}
	return "", fmt.Errorf("no Payload/*.app directory in zip")
}

// resolveSinfPaths returns the in-zip destinations for each sinf, preferring
// the Manifest.plist SinfPaths, falling back to SC_Info/<executable>.sinf.
func resolveSinfPaths(zr *zip.Reader, appDir string) ([]string, error) {
	if paths, ok := manifestSinfPaths(zr, appDir); ok {
		out := make([]string, len(paths))
		for i, p := range paths {
			out[i] = appDir + p
		}
		return out, nil
	}
	exe, err := bundleExecutable(zr, appDir)
	if err != nil {
		return nil, err
	}
	return []string{appDir + "SC_Info/" + exe + ".sinf"}, nil
}

func manifestSinfPaths(zr *zip.Reader, appDir string) ([]string, bool) {
	data, err := readZipFile(zr, appDir+"SC_Info/Manifest.plist")
	if err != nil {
		return nil, false
	}
	var m struct {
		SinfPaths []string `plist:"SinfPaths"`
	}
	if _, err := plist.Unmarshal(data, &m); err != nil || len(m.SinfPaths) == 0 {
		return nil, false
	}
	return m.SinfPaths, true
}

func bundleExecutable(zr *zip.Reader, appDir string) (string, error) {
	data, err := readZipFile(zr, appDir+"Info.plist")
	if err != nil {
		return "", fmt.Errorf("read Info.plist: %w", err)
	}
	var info struct {
		Executable string `plist:"CFBundleExecutable"`
	}
	if _, err := plist.Unmarshal(data, &info); err != nil {
		return "", fmt.Errorf("parse Info.plist: %w", err)
	}
	if info.Executable == "" {
		return "", fmt.Errorf("CFBundleExecutable missing from Info.plist")
	}
	return info.Executable, nil
}

func copyEntries(zw *zip.Writer, zr *zip.Reader) error {
	for _, f := range zr.File {
		if err := copyEntry(zw, f); err != nil {
			return err
		}
	}
	return nil
}

func copyEntry(zw *zip.Writer, f *zip.File) error {
	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("open %s: %w", f.Name, err)
	}
	defer func() { _ = rc.Close() }()

	hdr := f.FileHeader
	w, err := zw.CreateHeader(&hdr)
	if err != nil {
		return fmt.Errorf("create %s: %w", f.Name, err)
	}
	if f.FileInfo().IsDir() {
		return nil
	}
	if _, err := io.Copy(w, rc); err != nil {
		return fmt.Errorf("copy %s: %w", f.Name, err)
	}
	return nil
}

func writeSinfs(zw *zip.Writer, paths []string, sinfs []Sinf) error {
	for i, p := range paths {
		w, err := zw.Create(p)
		if err != nil {
			return fmt.Errorf("create sinf %s: %w", p, err)
		}
		if _, err := w.Write(sinfs[i].Data); err != nil {
			return fmt.Errorf("write sinf %s: %w", p, err)
		}
	}
	return nil
}

func writeMetadata(zw *zip.Writer, meta map[string]any, appleID string) error {
	if meta == nil {
		meta = map[string]any{}
	}
	if appleID != "" {
		meta["apple-id"] = appleID
		meta["userName"] = appleID
	}
	var buf bytes.Buffer
	enc := plist.NewEncoderForFormat(&buf, plist.XMLFormat)
	if err := enc.Encode(meta); err != nil {
		return fmt.Errorf("encode iTunesMetadata: %w", err)
	}
	w, err := zw.Create("iTunesMetadata.plist")
	if err != nil {
		return fmt.Errorf("create iTunesMetadata.plist: %w", err)
	}
	if _, err := io.Copy(w, &buf); err != nil {
		return fmt.Errorf("write iTunesMetadata.plist: %w", err)
	}
	return nil
}

func readZipFile(zr *zip.Reader, name string) ([]byte, error) {
	for _, f := range zr.File {
		if path.Clean(f.Name) == path.Clean(name) {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer func() { _ = rc.Close() }()
			return io.ReadAll(rc)
		}
	}
	return nil, fmt.Errorf("%s not found in zip", name)
}
