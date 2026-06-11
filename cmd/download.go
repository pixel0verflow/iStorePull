package cmd

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/pixel0verflow/istorepull/pkg/ipa"
	"github.com/pixel0verflow/istorepull/pkg/store"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
)

func newDownloadCmd() *cobra.Command {
	var (
		bundleID  string
		adamID    int64
		country   string
		versionID string
		version   string
		output    string
	)
	cmd := &cobra.Command{
		Use:   "download",
		Short: "Download an IPA (current build by default)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if versionID != "" && version != "" {
				return fmt.Errorf("%w: use only one of --version-id or --version", errBadInput)
			}
			ctx := cmd.Context()
			id, err := resolveAdamID(ctx, bundleID, adamID, country)
			if err != nil {
				return err
			}
			sess, err := loadSession()
			if err != nil {
				return err
			}
			client, err := store.New(sess)
			if err != nil {
				return err
			}

			extID, err := resolveExternalID(ctx, client, id, versionID, version)
			if err != nil {
				return err
			}

			item, err := client.DownloadProduct(ctx, id, extID)
			if err != nil {
				return err
			}

			dst := outputPath(output, bundleID, item)
			if err := fetchAndBuild(cmd, item, sess.AppleID, dst); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "saved %s (v%s)\n", dst, item.Version)
			return nil
		},
	}
	cmd.Flags().StringVarP(&bundleID, "bundle", "b", "", "bundle id")
	cmd.Flags().Int64VarP(&adamID, "id", "i", 0, "adam id")
	cmd.Flags().StringVar(&country, "country", "", "storefront country for bundle lookup")
	cmd.Flags().StringVar(&versionID, "version-id", "", "external version id to download")
	cmd.Flags().StringVar(&version, "version", "", "human version string to download (probes ids)")
	cmd.Flags().StringVarP(&output, "output", "o", "", "output path (file or directory)")
	return cmd
}

// resolveExternalID picks the external version id for the download.
func resolveExternalID(ctx context.Context, client store.Client, adamID int64, versionID, version string) (string, error) {
	switch {
	case versionID != "":
		return versionID, nil
	case version != "":
		vl, err := client.Versions(ctx, adamID)
		if err != nil {
			return "", err
		}
		return client.FindExternalID(ctx, vl, version)
	default:
		return "", nil // current build
	}
}

// outputPath resolves the destination file from -o (file, dir, or empty).
func outputPath(output, bundleID string, item store.DownloadItem) string {
	name := defaultName(bundleID, item)
	if output == "" {
		return name
	}
	if fi, err := os.Stat(output); err == nil && fi.IsDir() {
		return filepath.Join(output, name)
	}
	return output
}

func defaultName(bundleID string, item store.DownloadItem) string {
	base := bundleID
	if base == "" {
		base = metaName(item)
	}
	if base == "" {
		base = fmt.Sprintf("%d", item.AdamID)
	}
	ver := item.Version
	if ver == "" {
		ver = "unknown"
	}
	return fmt.Sprintf("%s_%s.ipa", base, ver)
}

func metaName(item store.DownloadItem) string {
	if item.Metadata == nil {
		return ""
	}
	if v, ok := item.Metadata["softwareVersionBundleId"].(string); ok {
		return v
	}
	return ""
}

// fetchAndBuild streams the asset to a temp file, verifies md5, and assembles
// the final IPA at dst.
func fetchAndBuild(cmd *cobra.Command, item store.DownloadItem, appleID, dst string) error {
	tmp, err := os.CreateTemp(absDir(dst), ".istorepull-*.zip")
	if err != nil {
		return fmt.Errorf("temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if err := streamTo(cmd, item.URL, tmp); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	if item.MD5 != "" {
		sum, err := md5File(tmpName)
		if err != nil {
			return err
		}
		if !equalFold(sum, item.MD5) {
			return fmt.Errorf("md5 mismatch: got %s want %s", sum, item.MD5)
		}
	}

	sinfs := make([]ipa.Sinf, len(item.Sinfs))
	for i, s := range item.Sinfs {
		sinfs[i] = ipa.Sinf{ID: s.ID, Data: s.Data}
	}
	return ipa.Build(ipa.BuildInput{
		SrcZip:   tmpName,
		DstPath:  dst,
		Sinfs:    sinfs,
		Metadata: item.Metadata,
		AppleID:  appleID,
	})
}

func streamTo(cmd *cobra.Command, url string, dst io.Writer) error {
	req, err := http.NewRequestWithContext(cmd.Context(), http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("download asset: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("asset CDN HTTP %d", resp.StatusCode)
	}

	bar := progressbar.DefaultBytes(resp.ContentLength, "downloading")
	_, err = io.Copy(io.MultiWriter(dst, bar), resp.Body)
	return err
}

func md5File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func absDir(path string) string {
	d := filepath.Dir(path)
	if d == "" {
		return "."
	}
	return d
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if 'A' <= ca && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if 'A' <= cb && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
