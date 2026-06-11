// Package store replays Apple's MZFinance store endpoints using a borrowed
// session: it lists downloadable versions and fetches IPA download tickets.
package store

// Sinf is a FairPlay supplemental info blob delivered with a download, injected
// into the IPA verbatim (no decryption).
type Sinf struct {
	ID   int64  `plist:"id"`
	Data []byte `plist:"sinf"`
}

// DownloadItem is one download ticket from a successful product response.
type DownloadItem struct {
	AdamID   int64
	URL      string
	MD5      string
	Sinfs    []Sinf
	Metadata map[string]any
	Version  string // metadata["bundleShortVersionString"]
	ExtVerID string // metadata["softwareVersionExternalIdentifier"]
}

// VersionList is the chronological set of downloadable builds for a title.
type VersionList struct {
	AdamID      int64
	Latest      string        // softwareVersionExternalIdentifier (current)
	ExternalIDs []string      // softwareVersionExternalIdentifiers (oldest -> newest)
	Resolved    []VersionInfo // optional; filled by ResolveVersions
}

// VersionInfo maps an external version id to a human version string.
type VersionInfo struct {
	ExternalID string
	Version    string
}

// songItem is the wire shape of a songList entry.
type songItem struct {
	URL      string         `plist:"URL"`
	MD5      string         `plist:"md5"`
	Sinfs    []Sinf         `plist:"sinfs"`
	Metadata map[string]any `plist:"metadata"`
}

// productResponse is the wire shape of a volumeStoreDownloadProduct reply.
type productResponse struct {
	FailureType     string     `plist:"failureType"`
	CustomerMessage string     `plist:"customerMessage"`
	JingleDocType   string     `plist:"jingleDocType"`
	SongList        []songItem `plist:"songList"`
}

// toDownloadItem converts a wire item into the public surface.
func (it songItem) toDownloadItem(adamID int64) DownloadItem {
	d := DownloadItem{
		AdamID:   adamID,
		URL:      it.URL,
		MD5:      it.MD5,
		Sinfs:    it.Sinfs,
		Metadata: it.Metadata,
	}
	d.Version = metaString(it.Metadata, "bundleShortVersionString")
	d.ExtVerID = metaString(it.Metadata, "softwareVersionExternalIdentifier")
	return d
}

func metaString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	switch v := m[key].(type) {
	case string:
		return v
	case int64:
		return itoa(v)
	case uint64:
		return itoa(int64(v))
	case int:
		return itoa(int64(v))
	default:
		return ""
	}
}
