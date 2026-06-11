package store

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/pixel0verflow/istorepull/pkg/httpx"
)

// versionServer answers probes by externalVersionId: returns the mapped version,
// or failureType 2059 for ids marked unavailable. Counts requests.
func versionServer(t *testing.T, idToVer map[string]string, unavailable map[string]bool, count *int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*count++
		body, _ := io.ReadAll(r.Body)
		var p downloadPayload
		if err := httpx.UnmarshalPlist(body, &p); err != nil {
			t.Errorf("decode payload: %v", err)
		}
		id := strconv.FormatInt(p.ExternalVersionID, 10)
		if unavailable[id] {
			_, _ = w.Write(plistResponse(t, map[string]any{"failureType": "2059"}))
			return
		}
		ver, ok := idToVer[id]
		if !ok {
			_, _ = w.Write(plistResponse(t, map[string]any{"failureType": "9610"}))
			return
		}
		_, _ = w.Write(plistResponse(t, map[string]any{
			"jingleDocType": "purchaseSuccess",
			"songList": []map[string]any{{
				"URL":      "https://cdn/x.zip",
				"metadata": map[string]any{"bundleShortVersionString": ver},
			}},
		}))
	}))
}

// monotonicList builds n ids with strictly increasing versions 17.0.0 .. 17.(n-1).0.
func monotonicList(n int) (ids []string, idToVer map[string]string) {
	idToVer = map[string]string{}
	for i := 0; i < n; i++ {
		id := strconv.Itoa(800000000 + i)
		ids = append(ids, id)
		idToVer[id] = fmt.Sprintf("17.%d.0", i)
	}
	return ids, idToVer
}

func TestFindExternalIDBinarySearch(t *testing.T) {
	ids, idToVer := monotonicList(200)
	var count int
	srv := versionServer(t, idToVer, nil, &count)
	defer srv.Close()

	c, _ := New(testSession(), WithBaseURL(srv.URL))
	vl := VersionList{AdamID: 1, ExternalIDs: ids}

	// target mid-list (index 97)
	got, err := c.FindExternalID(context.Background(), vl, "17.97.0")
	if err != nil {
		t.Fatalf("FindExternalID: %v", err)
	}
	if got != ids[97] {
		t.Errorf("got id %q, want %q", got, ids[97])
	}
	if count > 20 {
		t.Errorf("used %d probes for 200 builds — binary search should need ~log2(n)≈8, not linear", count)
	}
}

func TestFindExternalIDExactNewestAndOldest(t *testing.T) {
	ids, idToVer := monotonicList(64)
	var count int
	srv := versionServer(t, idToVer, nil, &count)
	defer srv.Close()
	c, _ := New(testSession(), WithBaseURL(srv.URL))
	vl := VersionList{AdamID: 1, ExternalIDs: ids}

	for _, tc := range []struct{ ver, want string }{
		{"17.0.0", ids[0]},
		{"17.63.0", ids[63]},
		{"17.31.0", ids[31]},
	} {
		got, err := c.FindExternalID(context.Background(), vl, tc.ver)
		if err != nil {
			t.Fatalf("%s: %v", tc.ver, err)
		}
		if got != tc.want {
			t.Errorf("%s -> %q, want %q", tc.ver, got, tc.want)
		}
	}
}

func TestFindExternalIDSkipsUnavailableMidpoint(t *testing.T) {
	ids, idToVer := monotonicList(32)
	// make the exact midpoints unavailable to force neighbour probing
	unavailable := map[string]bool{ids[16]: true, ids[15]: true}
	var count int
	srv := versionServer(t, idToVer, unavailable, &count)
	defer srv.Close()
	c, _ := New(testSession(), WithBaseURL(srv.URL))
	vl := VersionList{AdamID: 1, ExternalIDs: ids}

	got, err := c.FindExternalID(context.Background(), vl, "17.20.0")
	if err != nil {
		t.Fatalf("FindExternalID: %v", err)
	}
	if got != ids[20] {
		t.Errorf("got %q, want %q", got, ids[20])
	}
}

func TestFindExternalIDNotFound(t *testing.T) {
	ids, idToVer := monotonicList(16)
	var count int
	srv := versionServer(t, idToVer, nil, &count)
	defer srv.Close()
	c, _ := New(testSession(), WithBaseURL(srv.URL))
	vl := VersionList{AdamID: 1, ExternalIDs: ids}

	_, err := c.FindExternalID(context.Background(), vl, "99.99.99")
	if err == nil {
		t.Fatal("expected not-found error")
	}
}

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"17.4.0", "17.4.0", 0},
		{"17.4.0", "17.4.1", -1},
		{"17.5.1", "17.4.9", 1},
		{"17.4", "17.4.0", 0},
		{"18.0", "17.99.99", 1},
	}
	for _, tc := range cases {
		if got := compareVersions(parseVersion(tc.a), parseVersion(tc.b)); got != tc.want {
			t.Errorf("compare(%s,%s) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}
