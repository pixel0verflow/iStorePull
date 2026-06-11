// Package credential models a Configurator-captured Apple Store session and
// persists it to disk. iStorePull never mints tokens; it borrows a real session
// the user captured from Apple Configurator and replays it verbatim.
package credential

import (
	"errors"
	"net/http"
	"strings"
	"time"
)

// HTTPCookie is a single replayed cookie from the captured jar.
type HTTPCookie struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Domain   string `json:"domain"`
	Path     string `json:"path"`
	Secure   bool   `json:"secure"`
	HTTPOnly bool   `json:"httpOnly"`
}

// Session is a portable credential extracted from a Configurator capture.
//
// XToken is the real store credential (VPP/password token); it is session-scoped
// and expires. GUID is bound to XToken and must be reused verbatim.
type Session struct {
	XToken     string       `json:"xToken"`
	Cookies    []HTTPCookie `json:"cookies"`
	StoreFront string       `json:"storeFront"` // e.g. "143478-2,34"
	DSID       string       `json:"dsid"`
	GUID       string       `json:"guid"` // bound to XToken
	UserAgent  string       `json:"userAgent"`
	AppleID    string       `json:"appleID,omitempty"` // account email, optional (iTunesMetadata)
	CapturedAt time.Time    `json:"capturedAt"`
	Source     string       `json:"source"` // "charles:<file>", "paste", ...
}

// ErrIncomplete is returned by Valid when a required field is missing.
var ErrIncomplete = errors.New("incomplete session: missing required field")

// Valid reports whether the session carries the minimum fields needed to talk to
// the store.
func (s Session) Valid() error {
	switch {
	case strings.TrimSpace(s.XToken) == "":
		return wrap("xToken")
	case strings.TrimSpace(s.GUID) == "":
		return wrap("guid")
	case strings.TrimSpace(s.StoreFront) == "":
		return wrap("storeFront")
	case strings.TrimSpace(s.DSID) == "":
		return wrap("dsid")
	}
	return nil
}

func wrap(field string) error {
	return errIncompleteField{field: field}
}

type errIncompleteField struct{ field string }

func (e errIncompleteField) Error() string {
	return "incomplete session: missing required field " + e.field
}

func (e errIncompleteField) Is(target error) bool { return target == ErrIncomplete }

// Headers returns the request headers (excluding Cookie) the store expects.
func (s Session) Headers() map[string]string {
	h := map[string]string{
		"X-Token":             s.XToken,
		"X-Apple-Store-Front": s.StoreFront,
		"iCloud-DSID":         s.DSID,
		"X-Dsid":              s.DSID,
	}
	if s.UserAgent != "" {
		h["User-Agent"] = s.UserAgent
	}
	return h
}

// CookieHeader renders the captured jar as a single Cookie header value.
func (s Session) CookieHeader() string {
	parts := make([]string, 0, len(s.Cookies))
	for _, c := range s.Cookies {
		if c.Name == "" {
			continue
		}
		parts = append(parts, c.Name+"="+c.Value)
	}
	return strings.Join(parts, "; ")
}

// HTTPCookies converts the captured jar into net/http cookies for a cookiejar.
func (s Session) HTTPCookies() []*http.Cookie {
	out := make([]*http.Cookie, 0, len(s.Cookies))
	for _, c := range s.Cookies {
		if c.Name == "" {
			continue
		}
		out = append(out, &http.Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Secure:   c.Secure,
			HttpOnly: c.HTTPOnly,
		})
	}
	return out
}
