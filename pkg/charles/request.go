package charles

import (
	"net/http"

	"github.com/pixel0verflow/istorepull/pkg/credential"
)

// FromHTTPRequest builds a session from a live intercepted store request (used by
// the capture proxy). It reuses the same header/cookie/guid extraction as the
// file parsers.
func FromHTTPRequest(req *http.Request) (credential.Session, error) {
	f := flow{url: req.URL.String()}
	for name, vals := range req.Header {
		for _, v := range vals {
			f.headers = append(f.headers, header{name: name, value: v})
		}
	}
	return buildSession(f, "capture")
}
