package charles

import (
	"strings"

	"github.com/pixel0verflow/istorepull/pkg/credential"
)

// ParseHeaders builds a session from pasted raw request text. The input is a
// block of "Header: value" lines, optionally preceded by a request line
// ("POST https://buy.itunes.apple.com/...?guid=XXX HTTP/2") or a bare URL line
// that carries the guid query parameter.
func ParseHeaders(raw string) (credential.Session, error) {
	f := flow{}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimRight(line, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if u := urlFromLine(trimmed); u != "" && f.url == "" {
			f.url = u
		}
		name, value, found := strings.Cut(trimmed, ":")
		if !found {
			continue
		}
		name = strings.TrimSpace(name)
		value = strings.TrimSpace(value)
		if name == "" || strings.Contains(name, " ") {
			// Not a header line (e.g. a request line); skip.
			continue
		}
		f.headers = append(f.headers, header{name: name, value: value})
	}
	return buildSession(f, "paste")
}

// urlFromLine returns the first http(s) URL token found in a line, if any.
func urlFromLine(line string) string {
	for _, tok := range strings.Fields(line) {
		if strings.HasPrefix(tok, "http://") || strings.HasPrefix(tok, "https://") {
			return tok
		}
	}
	return ""
}
