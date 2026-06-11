package credential

import (
	"errors"
	"strings"
	"testing"
)

func sampleSession() Session {
	return Session{
		XToken:     "AwIAAAtoken",
		GUID:       "603E5F8B2D32",
		StoreFront: "143478-2,34",
		DSID:       "1092660217",
		UserAgent:  "Configurator/2.19",
		Cookies: []HTTPCookie{
			{Name: "mz_at_ssl-1092660217", Value: "abc"},
			{Name: "itspod", Value: "42"},
		},
	}
}

func TestValid(t *testing.T) {
	if err := sampleSession().Valid(); err != nil {
		t.Fatalf("expected valid session, got %v", err)
	}

	tests := map[string]func(*Session){
		"xToken":     func(s *Session) { s.XToken = "" },
		"guid":       func(s *Session) { s.GUID = "" },
		"storeFront": func(s *Session) { s.StoreFront = "" },
		"dsid":       func(s *Session) { s.DSID = "" },
	}
	for field, mut := range tests {
		s := sampleSession()
		mut(&s)
		err := s.Valid()
		if err == nil {
			t.Errorf("%s: expected error, got nil", field)
			continue
		}
		if !errors.Is(err, ErrIncomplete) {
			t.Errorf("%s: expected ErrIncomplete, got %v", field, err)
		}
		if !strings.Contains(err.Error(), field) {
			t.Errorf("%s: error should name the field, got %q", field, err.Error())
		}
	}
}

func TestHeaders(t *testing.T) {
	h := sampleSession().Headers()
	want := map[string]string{
		"X-Token":             "AwIAAAtoken",
		"X-Apple-Store-Front": "143478-2,34",
		"iCloud-DSID":         "1092660217",
		"X-Dsid":              "1092660217",
		"User-Agent":          "Configurator/2.19",
	}
	for k, v := range want {
		if h[k] != v {
			t.Errorf("header %s = %q, want %q", k, h[k], v)
		}
	}
}

func TestCookieHeader(t *testing.T) {
	got := sampleSession().CookieHeader()
	if got != "mz_at_ssl-1092660217=abc; itspod=42" {
		t.Errorf("cookie header = %q", got)
	}
}

func TestHTTPCookies(t *testing.T) {
	cs := sampleSession().HTTPCookies()
	if len(cs) != 2 {
		t.Fatalf("expected 2 cookies, got %d", len(cs))
	}
	if cs[0].Name != "mz_at_ssl-1092660217" || cs[0].Value != "abc" {
		t.Errorf("unexpected first cookie %+v", cs[0])
	}
}
