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
// selective-MITM proxy, waits for Apple Configurator to make a store request,
// extracts the session, and tears everything back down.
func Run(ctx context.Context, sys System, opts Options, out io.Writer) (credential.Session, error) {
	if opts.Addr == "" {
		opts.Addr = "127.0.0.1:0"
	}
	if opts.Timeout == 0 {
		opts.Timeout = 5 * time.Minute
	}

	ca, err := NewCA()
	if err != nil {
		return credential.Session{}, err
	}

	removeCA, err := sys.TrustCA(ca.CertPEM())
	if err != nil {
		return credential.Session{}, fmt.Errorf("trust CA: %w", err)
	}
	if !opts.KeepCA && removeCA != nil {
		defer func() { _ = removeCA() }()
	}

	ln, err := net.Listen("tcp", opts.Addr)
	if err != nil {
		return credential.Session{}, fmt.Errorf("listen: %w", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	restore, err := sys.SetProxy("127.0.0.1", port)
	if err != nil {
		_ = ln.Close()
		return credential.Session{}, fmt.Errorf("set system proxy: %w", err)
	}
	if restore != nil {
		defer func() { _ = restore() }()
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

	fmt.Fprintf(out, "capture proxy listening on 127.0.0.1:%d\n", port)
	fmt.Fprintln(out, "→ In Apple Configurator, download or update any app on your device now…")

	wait, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	select {
	case s := <-sessCh:
		_ = ln.Close()
		return s, nil
	case <-wait.Done():
		_ = ln.Close()
		return credential.Session{}, fmt.Errorf("timed out after %s waiting for a store request — was a download triggered in Configurator?", opts.Timeout)
	}
}
