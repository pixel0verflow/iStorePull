package capture

import (
	"bufio"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
)

// Proxy is a minimal HTTP CONNECT proxy that decrypts the store hosts and
// tunnels everything else. It is not a general-purpose proxy — it exists only to
// observe Configurator's store request and extract the session headers.
type Proxy struct {
	ca *CA

	// onRequest is invoked (headers only) for each decrypted store request.
	onRequest func(*http.Request)
	// logf receives diagnostic lines (may be nil).
	logf func(string, ...any)

	// dialTLS dials an upstream store host over TLS. Overridable for tests.
	dialTLS func(host string) (net.Conn, error)
	// dialRaw dials a raw upstream for tunnelled hosts. Overridable for tests.
	dialRaw func(hostport string) (net.Conn, error)

	wg sync.WaitGroup
}

// newProxy builds a proxy with default (real) upstream dialers.
func newProxy(ca *CA, onRequest func(*http.Request)) *Proxy {
	return &Proxy{
		ca:        ca,
		onRequest: onRequest,
		dialTLS: func(host string) (net.Conn, error) {
			return tls.Dial("tcp", host+":443", &tls.Config{
				ServerName: host,
				MinVersion: tls.VersionTLS12,
				NextProtos: []string{"http/1.1"},
			})
		},
		dialRaw: func(hostport string) (net.Conn, error) {
			return net.Dial("tcp", hostport)
		},
	}
}

func (p *Proxy) log(format string, args ...any) {
	if p.logf != nil {
		p.logf(format, args...)
	}
}

// Serve accepts connections until ln is closed.
func (p *Proxy) Serve(ln net.Listener) error {
	for {
		conn, err := ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				p.wg.Wait()
				return nil
			}
			return err
		}
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			p.handle(conn)
		}()
	}
}

func (p *Proxy) handle(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	br := bufio.NewReader(conn)
	req, err := http.ReadRequest(br)
	if err != nil {
		return
	}
	if req.Method != http.MethodConnect {
		// We only handle CONNECT (HTTPS). Plain HTTP is not used by the store.
		return
	}

	host := stripPort(req.Host)
	if _, err := io.WriteString(conn, "HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		return
	}

	if shouldMITM(host) {
		p.mitm(conn, host)
		return
	}
	p.tunnel(conn, req.Host)
}

// tunnel splices a raw connection to the upstream host (no decryption).
func (p *Proxy) tunnel(client net.Conn, hostport string) {
	if !strings.Contains(hostport, ":") {
		hostport += ":443"
	}
	upstream, err := p.dialRaw(hostport)
	if err != nil {
		p.log("tunnel dial %s: %v", hostport, err)
		return
	}
	defer func() { _ = upstream.Close() }()

	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(upstream, client); done <- struct{}{} }()
	go func() { _, _ = io.Copy(client, upstream); done <- struct{}{} }()
	<-done
}

// mitm terminates TLS with a minted leaf, reads the request(s), hands the headers
// to onRequest, and forwards transparently to the real host.
func (p *Proxy) mitm(client net.Conn, host string) {
	tlsConn := tls.Server(client, p.ca.serverTLSConfig())
	if err := tlsConn.Handshake(); err != nil {
		p.log("tls handshake %s: %v", host, err)
		return
	}
	defer func() { _ = tlsConn.Close() }()

	br := bufio.NewReader(tlsConn)
	for {
		req, err := http.ReadRequest(br)
		if err != nil {
			return
		}
		req.URL.Scheme = "https"
		req.URL.Host = host

		if p.onRequest != nil {
			p.onRequest(cloneForInspection(req))
		}

		if err := p.forward(tlsConn, host, req); err != nil {
			p.log("forward %s: %v", host, err)
			return
		}
		if req.Close {
			return
		}
	}
}

// forward sends req to the upstream store host and copies the response back.
func (p *Proxy) forward(client net.Conn, host string, req *http.Request) error {
	upstream, err := p.dialTLS(host)
	if err != nil {
		return fmt.Errorf("dial upstream: %w", err)
	}
	defer func() { _ = upstream.Close() }()

	req.RequestURI = ""
	if err := req.Write(upstream); err != nil {
		return fmt.Errorf("write upstream: %w", err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(upstream), req)
	if err != nil {
		return fmt.Errorf("read upstream: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.Write(client)
}

// cloneForInspection copies the request's headers and URL so onRequest can read
// them without disturbing the body that forward streams upstream.
func cloneForInspection(req *http.Request) *http.Request {
	clone := &http.Request{
		Method: req.Method,
		URL:    req.URL,
		Header: req.Header.Clone(),
		Host:   req.Host,
	}
	return clone
}
