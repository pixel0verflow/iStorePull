//go:build !darwin

package capture

// NewSystem returns a System that refuses to run on non-macOS platforms.
func NewSystem() System { return unsupportedSystem{} }

type unsupportedSystem struct{}

func (unsupportedSystem) TrustCA([]byte) (func() error, error)       { return nil, ErrUnsupported }
func (unsupportedSystem) SetProxy(string, int) (func() error, error) { return nil, ErrUnsupported }
