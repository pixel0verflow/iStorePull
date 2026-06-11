// Package httpx is the HTTP transport for store replay: a cookie-jar client that
// preserves POST bodies across Apple's pod redirects and speaks x-apple-plist.
package httpx

import (
	"bytes"

	"howett.net/plist"
)

// MarshalPlist encodes v as an XML plist body.
func MarshalPlist(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := plist.NewEncoderForFormat(&buf, plist.XMLFormat)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// UnmarshalPlist decodes an XML/binary plist into v. Apple sometimes prefixes
// store responses with stray bytes before the plist; sanitize trims to the first
// recognizable plist marker.
func UnmarshalPlist(data []byte, v any) error {
	clean := sanitize(data)
	_, err := plist.Unmarshal(clean, v)
	return err
}

// sanitize trims leading garbage so the plist decoder sees a clean document.
func sanitize(data []byte) []byte {
	if bytes.HasPrefix(data, []byte("bplist")) {
		return data
	}
	if i := bytes.Index(data, []byte("<?xml")); i > 0 {
		return data[i:]
	}
	if i := bytes.Index(data, []byte("<plist")); i > 0 {
		return data[i:]
	}
	return data
}
