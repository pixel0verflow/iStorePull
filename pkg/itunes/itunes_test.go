package itunes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const lookupBody = `{"resultCount":1,"results":[
  {"trackId":357218860,"bundleId":"com.example.app","trackName":"Example","version":"17.4.0","sellerName":"Example Inc"}
]}`

const searchBody = `{"resultCount":2,"results":[
  {"trackId":1,"bundleId":"com.a","trackName":"A"},
  {"trackId":2,"bundleId":"com.b","trackName":"B"}
]}`

func TestLookupBundle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "bundleId=com.example.app") {
			t.Errorf("query missing bundleId: %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(lookupBody))
	}))
	defer srv.Close()

	app, err := New(WithBaseURL(srv.URL)).LookupBundle(context.Background(), "com.example.app", "us")
	if err != nil {
		t.Fatalf("LookupBundle: %v", err)
	}
	if app.ID != 357218860 || app.BundleID != "com.example.app" {
		t.Errorf("unexpected app: %+v", app)
	}
}

func TestLookupID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "id=357218860") {
			t.Errorf("query missing id: %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(lookupBody))
	}))
	defer srv.Close()

	app, err := New(WithBaseURL(srv.URL)).LookupID(context.Background(), 357218860, "")
	if err != nil {
		t.Fatalf("LookupID: %v", err)
	}
	if app.BundleID != "com.example.app" {
		t.Errorf("bundle = %q", app.BundleID)
	}
}

func TestSearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "term=hello+world") {
			t.Errorf("query missing term: %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(searchBody))
	}))
	defer srv.Close()

	apps, err := New(WithBaseURL(srv.URL)).Search(context.Background(), "hello world", "", 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(apps) != 2 {
		t.Fatalf("results = %d, want 2", len(apps))
	}
}

func TestLookupNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"resultCount":0,"results":[]}`))
	}))
	defer srv.Close()

	_, err := New(WithBaseURL(srv.URL)).LookupBundle(context.Background(), "com.missing", "")
	if err == nil {
		t.Fatal("expected no-result error")
	}
}
