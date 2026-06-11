package charles

import (
	"net/http"
	"testing"
)

func TestFromHTTPRequest(t *testing.T) {
	req, _ := http.NewRequest(http.MethodPost,
		"https://buy.itunes.apple.com/WebObjects/MZFinance.woa/wa/volumeStoreDownloadProduct?guid=603E5F8B2D32", nil)
	req.Header.Set("X-Token", "AwIAAAtoken==")
	req.Header.Set("X-Apple-Store-Front", "143478-2,34")
	req.Header.Set("iCloud-DSID", "1092660217")
	req.Header.Set("User-Agent", "Configurator/2.19")
	req.Header.Set("Cookie", "mz_at_ssl-1092660217=abc; itspod=42; mt-tkn-1092660217=xyz")

	sess, err := FromHTTPRequest(req)
	if err != nil {
		t.Fatalf("FromHTTPRequest: %v", err)
	}
	assertSampleSession(t, sess)
	if sess.Source != "capture" {
		t.Errorf("source = %q, want capture", sess.Source)
	}
}

func TestFromHTTPRequestNoToken(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "https://buy.itunes.apple.com/x?guid=G", nil)
	req.Header.Set("X-Apple-Store-Front", "143478")
	if _, err := FromHTTPRequest(req); err == nil {
		t.Fatal("expected error without X-Token")
	}
}
