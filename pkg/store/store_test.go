package store

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pixel0verflow/istorepull/pkg/credential"
	"github.com/pixel0verflow/istorepull/pkg/httpx"
)

func testSession() credential.Session {
	return credential.Session{
		XToken:     "tok",
		GUID:       "GUID123",
		StoreFront: "143478-2,34",
		DSID:       "1092660217",
		UserAgent:  "Configurator/2.19",
	}
}

func plistResponse(t *testing.T, v any) []byte {
	t.Helper()
	b, err := httpx.MarshalPlist(v)
	if err != nil {
		t.Fatalf("marshal plist: %v", err)
	}
	return b
}

func successBody(t *testing.T) []byte {
	return plistResponse(t, map[string]any{
		"jingleDocType": "purchaseSuccess",
		"songList": []map[string]any{{
			"URL": "https://cdn.example/app.zip",
			"md5": "deadbeef",
			"sinfs": []map[string]any{
				{"id": 0, "sinf": []byte("SINF")},
			},
			"metadata": map[string]any{
				"bundleShortVersionString":           "17.4.0",
				"softwareVersionExternalIdentifier":  871517989,
				"softwareVersionExternalIdentifiers": []any{int64(870000000), int64(871517989)},
				"softwareVersionBundleId":            "com.example.app",
			},
		}},
	})
}

func TestDownloadProductPodRedirect(t *testing.T) {
	var podBody string
	var podMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "volumeStoreDownloadProduct"):
			http.Redirect(w, r, "/pod/get", http.StatusFound)
		case r.URL.Path == "/pod/get":
			b, _ := io.ReadAll(r.Body)
			podBody = string(b)
			podMethod = r.Method
			_, _ = w.Write(successBody(t))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c, err := New(testSession(), WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	item, err := c.DownloadProduct(context.Background(), 357218860, "")
	if err != nil {
		t.Fatalf("DownloadProduct: %v", err)
	}

	if podMethod != http.MethodPost {
		t.Errorf("pod request method = %s, want POST (body must survive redirect)", podMethod)
	}
	if !strings.Contains(podBody, "salableAdamId") {
		t.Errorf("pod request body lost the plist payload: %q", podBody)
	}
	if item.Version != "17.4.0" {
		t.Errorf("version = %q, want 17.4.0", item.Version)
	}
	if item.ExtVerID != "871517989" {
		t.Errorf("extVerID = %q, want 871517989", item.ExtVerID)
	}
	if len(item.Sinfs) != 1 || string(item.Sinfs[0].Data) != "SINF" {
		t.Errorf("sinfs not decoded: %+v", item.Sinfs)
	}
	if item.URL != "https://cdn.example/app.zip" {
		t.Errorf("url = %q", item.URL)
	}
}

func TestDownloadProductFailureExpired(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(plistResponse(t, map[string]any{
			"failureType":     "2042",
			"customerMessage": "Sign In to the iTunes Store",
		}))
	}))
	defer srv.Close()

	c, _ := New(testSession(), WithBaseURL(srv.URL))
	_, err := c.DownloadProduct(context.Background(), 1, "")
	if !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("expected ErrSessionExpired, got %v", err)
	}
}

func TestDownloadProductNoLicense(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(plistResponse(t, map[string]any{"failureType": "9610"}))
	}))
	defer srv.Close()

	c, _ := New(testSession(), WithBaseURL(srv.URL))
	_, err := c.DownloadProduct(context.Background(), 1, "")
	if !errors.Is(err, ErrNoLicense) {
		t.Fatalf("expected ErrNoLicense, got %v", err)
	}
}

func TestVersions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(successBody(t))
	}))
	defer srv.Close()

	c, _ := New(testSession(), WithBaseURL(srv.URL))
	vl, err := c.Versions(context.Background(), 357218860)
	if err != nil {
		t.Fatalf("Versions: %v", err)
	}
	if vl.Latest != "871517989" {
		t.Errorf("latest = %q", vl.Latest)
	}
	want := []string{"870000000", "871517989"}
	if len(vl.ExternalIDs) != len(want) {
		t.Fatalf("ids = %v, want %v", vl.ExternalIDs, want)
	}
	for i := range want {
		if vl.ExternalIDs[i] != want[i] {
			t.Errorf("id[%d] = %q, want %q", i, vl.ExternalIDs[i], want[i])
		}
	}
}

func TestExternalVersionIDInPayload(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		_, _ = w.Write(successBody(t))
	}))
	defer srv.Close()

	c, _ := New(testSession(), WithBaseURL(srv.URL))
	if _, err := c.DownloadProduct(context.Background(), 1, "878571262"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotBody, "externalVersionId") || !strings.Contains(gotBody, "878571262") {
		t.Errorf("externalVersionId not in body: %q", gotBody)
	}
}
