package capture

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/pixel0verflow/istorepull/pkg/charles"
	"github.com/pixel0verflow/istorepull/pkg/credential"
)

// Options configures a capture run.
type Options struct {
	Addr    string        // proxy listen address (default 127.0.0.1:0)
	Timeout time.Duration // how long to wait for a store request (default 5m)
	KeepCA  bool          // leave the CA trusted after capture
	Verbose bool
}

// Run installs the throwaway CA, points the system proxy at an embedded
// selective-MITM proxy, and waits for Apple Configurator to make a store
// request. On success it returns the extracted session plus a cleanup func the
// caller must invoke (after saving) to restore the proxy and remove the
// certificate. On error, teardown has already run and cleanup is a no-op.
func Run(ctx context.Context, sys System, opts Options, out io.Writer) (credential.Session, func(), error) {
	noop := func() {}
	if opts.Addr == "" {
		opts.Addr = "127.0.0.1:0"
	}
	if opts.Timeout == 0 {
		opts.Timeout = 5 * time.Minute
	}

	fmt.Fprintln(out, "starting capture")

	ca, err := NewCA()
	if err != nil {
		return credential.Session{}, noop, err
	}

	fmt.Fprintln(out, "adding temporary root certificate to keychain (admin prompt)")
	removeCA, err := sys.TrustCA(ca.CertPEM())
	if err != nil {
		return credential.Session{}, noop, fmt.Errorf("trust CA: %w", err)
	}

	ln, err := net.Listen("tcp", opts.Addr)
	if err != nil {
		removeCert(out, opts, removeCA)
		return credential.Session{}, noop, fmt.Errorf("listen: %w", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	restore, err := sys.SetProxy("127.0.0.1", port)
	if err != nil {
		_ = ln.Close()
		removeCert(out, opts, removeCA)
		return credential.Session{}, noop, fmt.Errorf("set system proxy: %w", err)
	}

	cleanup := func() {
		if restore != nil {
			_ = restore()
		}
		removeCert(out, opts, removeCA)
	}

	sessCh := make(chan credential.Session, 1)
	var once sync.Once
	proxy := newProxy(ca, func(req *http.Request) {
		s, err := charles.FromHTTPRequest(req)
		if err != nil {
			return
		}
		once.Do(func() { sessCh <- s })
	})
	if opts.Verbose {
		proxy.logf = func(format string, args ...any) { fmt.Fprintf(out, "  "+format+"\n", args...) }
	}

	go func() { _ = proxy.Serve(ln) }()

	fmt.Fprintln(out, "waiting for Configurator download")

	wait, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	select {
	case s := <-sessCh:
		_ = ln.Close()
		return s, cleanup, nil
	case <-wait.Done():
		_ = ln.Close()
		cleanup()
		return credential.Session{}, noop, fmt.Errorf("timed out after %s waiting for a store request — was a download triggered in Configurator?", opts.Timeout)
	}
}

// removeCert removes the trusted CA unless the user asked to keep it.
func removeCert(out io.Writer, opts Options, removeCA func() error) {
	if opts.KeepCA || removeCA == nil {
		return
	}
	fmt.Fprintln(out, "removing root certificate from keychain")
	_ = removeCA()
}
