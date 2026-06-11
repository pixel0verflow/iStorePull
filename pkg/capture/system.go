package capture

import "errors"

// ErrUnsupported is returned by the system integration on non-macOS platforms.
var ErrUnsupported = errors.New("automated capture is only supported on macOS (Apple Configurator is macOS-only)")

// System abstracts the OS-level side effects capture needs: trusting the
// throwaway CA and pointing the system HTTPS proxy at us. Kept behind an
// interface so the capture flow is testable without touching the keychain or
// network settings.
type System interface {
	// TrustCA installs the PEM-encoded CA as a trusted root. Returns a function
	// that removes it again (called on teardown unless the user keeps it).
	TrustCA(pemBytes []byte) (remove func() error, err error)
	// SetProxy points the system secure-web proxy at host:port and returns a
	// function that restores the previous settings.
	SetProxy(host string, port int) (restore func() error, err error)
}
