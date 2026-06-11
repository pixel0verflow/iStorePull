package charles

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"github.com/pixel0verflow/istorepull/pkg/credential"
)

const pastedHeaders = `POST https://buy.itunes.apple.com/WebObjects/MZFinance.woa/wa/volumeStoreDownloadProduct?guid=603E5F8B2D32 HTTP/2
Host: buy.itunes.apple.com
X-Token: AwIAAAtoken==
X-Apple-Store-Front: 143478-2,34
iCloud-DSID: 1092660217
X-Dsid: 1092660217
User-Agent: Configurator/2.19 (Macintosh; OS X 26.1)
Cookie: mz_at_ssl-1092660217=abc; itspod=42; mt-tkn-1092660217=xyz
Content-Type: application/x-apple-plist`

func assertSampleSession(t *testing.T, s credential.Session) {
	t.Helper()
	if s.XToken != "AwIAAAtoken==" {
		t.Errorf("X-Token = %q", s.XToken)
	}
	if s.GUID != "603E5F8B2D32" {
		t.Errorf("guid = %q", s.GUID)
	}
	if s.StoreFront != "143478-2,34" {
		t.Errorf("storefront = %q", s.StoreFront)
	}
	if s.DSID != "1092660217" {
		t.Errorf("dsid = %q", s.DSID)
	}
	if len(s.Cookies) != 3 {
		t.Errorf("cookies = %d, want 3", len(s.Cookies))
	}
}

func TestParseHeaders(t *testing.T) {
	sess, err := ParseHeaders(pastedHeaders)
	if err != nil {
		t.Fatalf("ParseHeaders: %v", err)
	}
	assertSampleSession(t, sess)
	if sess.UserAgent != "Configurator/2.19 (Macintosh; OS X 26.1)" {
		t.Errorf("user-agent = %q", sess.UserAgent)
	}
}

func TestParseHeadersNoToken(t *testing.T) {
	_, err := ParseHeaders("Host: example.com\nCookie: a=b")
	if err == nil {
		t.Fatal("expected error for missing token")
	}
}

const sampleHAR = `{
  "log": { "entries": [
    { "request": { "url": "https://gsa.apple.com/grandslam", "headers": [{"name":"X-Apple-MD","value":"x"}] } },
    { "request": {
      "url": "https://buy.itunes.apple.com/WebObjects/MZFinance.woa/wa/volumeStoreDownloadProduct?guid=603E5F8B2D32",
      "headers": [
        {"name":"X-Token","value":"AwIAAAtoken=="},
        {"name":"X-Apple-Store-Front","value":"143478-2,34"},
        {"name":"iCloud-DSID","value":"1092660217"},
        {"name":"User-Agent","value":"Configurator/2.19"},
        {"name":"Cookie","value":"mz_at_ssl-1092660217=abc; itspod=42; mt-tkn-1092660217=xyz"}
      ]
    } }
  ] }
}`

func TestParseHAR(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dump.har")
	if err := os.WriteFile(path, []byte(sampleHAR), 0o600); err != nil {
		t.Fatal(err)
	}
	sess, err := ParseHAR(path)
	if err != nil {
		t.Fatalf("ParseHAR: %v", err)
	}
	assertSampleSession(t, sess)
}

const sampleMeta = `{
  "host": "buy.itunes.apple.com",
  "path": "/WebObjects/MZFinance.woa/wa/volumeStoreDownloadProduct",
  "query": "guid=603E5F8B2D32",
  "request": { "header": { "headers": [
    {"name":"X-Token","value":"AwIAAAtoken=="},
    {"name":"X-Apple-Store-Front","value":"143478-2,34"},
    {"name":"iCloud-DSID","value":"1092660217"},
    {"name":"User-Agent","value":"Configurator/2.19"},
    {"name":"Cookie","value":"mz_at_ssl-1092660217=abc; itspod=42; mt-tkn-1092660217=xyz"}
  ] } }
}`

func TestParseDumpCHLZ(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.chlz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	// a noise flow plus the store flow
	w, _ := zw.Create("0-meta.json")
	_, _ = w.Write([]byte(`{"host":"gsa.apple.com","path":"/x","request":{"header":{"headers":[]}}}`))
	w, _ = zw.Create("1-meta.json")
	_, _ = w.Write([]byte(sampleMeta))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	sess, err := ParseDump(path)
	if err != nil {
		t.Fatalf("ParseDump: %v", err)
	}
	assertSampleSession(t, sess)
}
