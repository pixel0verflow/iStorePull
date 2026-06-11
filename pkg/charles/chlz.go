package charles

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/pixel0verflow/istorepull/pkg/credential"
)

// metaFile is the subset of a Charles "N-meta.json" per-flow file we read.
// Charles nests request headers under request.header.headers.
type metaFile struct {
	Host   string `json:"host"`
	Path   string `json:"path"`
	Query  string `json:"query"`
	Scheme string `json:"scheme"`
	URL    string `json:"url"`

	Request struct {
		Header struct {
			Headers []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"headers"`
		} `json:"header"`
	} `json:"request"`
}

// ParseDump dispatches by file extension. ".chlz"/".chls" are treated as the
// ZIP-of-meta format; ".har"/".json" fall through to ParseHAR.
func ParseDump(path string) (credential.Session, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".har", ".json":
		return ParseHAR(path)
	default:
		return parseCHLZ(path)
	}
}

// parseCHLZ reads a Charles session archive (ZIP of *-meta.json flow files).
func parseCHLZ(path string) (credential.Session, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return credential.Session{}, fmt.Errorf("open %s as zip: %w", path, err)
	}
	defer func() { _ = zr.Close() }()

	var flows []flow
	for _, zf := range zr.File {
		if !strings.HasSuffix(zf.Name, "-meta.json") && filepath.Base(zf.Name) != "meta.json" {
			continue
		}
		f, ok := flowFromMeta(zf)
		if ok {
			flows = append(flows, f)
		}
	}

	picked, ok := pickFlow(flows)
	if !ok {
		return credential.Session{}, ErrNoStoreFlow
	}
	return buildSession(picked, "charles:"+filepath.Base(path))
}

func flowFromMeta(zf *zip.File) (flow, bool) {
	rc, err := zf.Open()
	if err != nil {
		return flow{}, false
	}
	defer func() { _ = rc.Close() }()

	data, err := io.ReadAll(rc)
	if err != nil {
		return flow{}, false
	}
	var m metaFile
	if err := json.Unmarshal(data, &m); err != nil {
		return flow{}, false
	}

	f := flow{url: metaURL(m)}
	for _, hd := range m.Request.Header.Headers {
		f.headers = append(f.headers, header{name: hd.Name, value: hd.Value})
	}
	return f, true
}

// metaURL reconstructs a URL from a meta file, preferring an explicit url field.
func metaURL(m metaFile) string {
	if m.URL != "" {
		return m.URL
	}
	scheme := m.Scheme
	if scheme == "" {
		scheme = "https"
	}
	u := scheme + "://" + m.Host + m.Path
	if m.Query != "" {
		u += "?" + m.Query
	}
	return u
}
