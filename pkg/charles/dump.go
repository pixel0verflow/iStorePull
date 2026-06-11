// Package charles extracts an iStorePull session from a captured proxy flow.
//
// Three input shapes are supported:
//   - ParseDump: a Charles ".chlz"/".chls" session (ZIP of per-flow files).
//   - ParseHAR:  a standard HAR JSON export (mitmproxy/Proxyman/Charles).
//   - ParseHeaders: raw pasted request headers (plus the request line/URL).
//
// All three converge on buildSession, which pulls X-Token, the cookie jar,
// X-Apple-Store-Front, the DSID and the bound guid out of a store request.
package charles

import (
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/pixel0verflow/istorepull/pkg/credential"
)

// ErrNoStoreFlow is returned when no usable store request is found in a capture.
var ErrNoStoreFlow = errors.New("no store request with an X-Token found in capture")

// storePathMarkers identify a flow that carries the headers we need.
var storePathMarkers = []string{
	"volumeStoreDownloadProduct",
	"DownloadDispatch.woa/wa/ent/download",
	"buyProduct",
	"MZFinance.woa",
}

// header is one captured request header.
type header struct {
	name  string
	value string
}

// flow is the minimal request shape the parsers extract.
type flow struct {
	url     string
	headers []header
}

// looksLikeStore reports whether the URL path matches a known store endpoint.
func looksLikeStore(rawurl string) bool {
	for _, m := range storePathMarkers {
		if strings.Contains(rawurl, m) {
			return true
		}
	}
	return false
}

// hasXToken reports whether the flow carries an X-Token request header.
func (f flow) hasXToken() bool {
	for _, h := range f.headers {
		if strings.EqualFold(h.name, "X-Token") && strings.TrimSpace(h.value) != "" {
			return true
		}
	}
	return false
}

// pickFlow selects the best store flow: prefer one matching a store path AND
// carrying an X-Token; fall back to any flow carrying an X-Token.
func pickFlow(flows []flow) (flow, bool) {
	var fallback flow
	haveFallback := false
	for _, f := range flows {
		if !f.hasXToken() {
			continue
		}
		if looksLikeStore(f.url) {
			return f, true
		}
		if !haveFallback {
			fallback, haveFallback = f, true
		}
	}
	return fallback, haveFallback
}

// buildSession maps a chosen flow onto a credential.Session.
func buildSession(f flow, source string) (credential.Session, error) {
	s := credential.Session{Source: source, CapturedAt: time.Now().UTC()}

	for _, h := range f.headers {
		switch {
		case strings.EqualFold(h.name, "X-Token"):
			s.XToken = strings.TrimSpace(h.value)
		case strings.EqualFold(h.name, "X-Apple-Store-Front"):
			s.StoreFront = strings.TrimSpace(h.value)
		case strings.EqualFold(h.name, "iCloud-DSID"), strings.EqualFold(h.name, "X-Dsid"):
			if s.DSID == "" {
				s.DSID = strings.TrimSpace(h.value)
			}
		case strings.EqualFold(h.name, "User-Agent"):
			s.UserAgent = strings.TrimSpace(h.value)
		case strings.EqualFold(h.name, "Cookie"):
			s.Cookies = append(s.Cookies, parseCookies(h.value)...)
		}
	}

	s.GUID = guidFromURL(f.url)
	if err := s.Valid(); err != nil {
		return s, err
	}
	return s, nil
}

// guidFromURL extracts the guid query parameter from a store URL.
func guidFromURL(rawurl string) string {
	u, err := url.Parse(rawurl)
	if err != nil {
		return ""
	}
	return u.Query().Get("guid")
}

// parseCookies splits a Cookie header value into individual cookies, scoped to
// .apple.com (where the store sets them).
func parseCookies(value string) []credential.HTTPCookie {
	var out []credential.HTTPCookie
	for _, part := range strings.Split(value, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, val, found := strings.Cut(part, "=")
		if !found {
			continue
		}
		out = append(out, credential.HTTPCookie{
			Name:     strings.TrimSpace(name),
			Value:    strings.TrimSpace(val),
			Domain:   ".apple.com",
			Path:     "/",
			Secure:   true,
			HTTPOnly: true,
		})
	}
	return out
}
