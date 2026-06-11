package capture

import "strings"

// storeHosts are the hosts we decrypt: the store front and the pods. Everything
// else — most importantly the certificate-pinned gsa.apple.com — is tunnelled
// through untouched so Configurator's own auth keeps working.
var storeExact = map[string]bool{
	"buy.itunes.apple.com":              true,
	"downloaddispatch.itunes.apple.com": true,
}

// shouldMITM reports whether a host should be intercepted (decrypted) rather
// than tunnelled raw.
func shouldMITM(host string) bool {
	host = strings.ToLower(stripPort(host))
	if storeExact[host] {
		return true
	}
	// pod hosts: pN-buy.itunes.apple.com
	return strings.HasSuffix(host, "-buy.itunes.apple.com")
}

// stripPort removes a trailing :port from a host[:port].
func stripPort(host string) string {
	if i := strings.LastIndex(host, ":"); i >= 0 {
		// guard against IPv6 literals without brackets; store hosts are names.
		if !strings.Contains(host[i+1:], ":") {
			return host[:i]
		}
	}
	return host
}
