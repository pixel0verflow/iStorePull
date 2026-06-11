package capture

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pixel0verflow/istorepull/pkg/charles"
)

// startProxy runs a proxy on a fresh listener and returns its address + a stop fn.
func startProxy(t *testing.T, p *Proxy) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = p.Serve(ln) }()
	return ln.Addr().String(), func() { _ = ln.Close() }
}

// connectAndTLS opens a CONNECT tunnel through the proxy and, for MITM hosts,
// completes a TLS handshake trusting the proxy CA.
func connectThroughProxy(t *testing.T, proxyAddr, host string) net.Conn {
	t.Helper()
	conn, err := net.Dial("tcp", proxyAddr)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprintf(conn, "CONNECT %s:443 HTTP/1.1\r\nHost: %s:443\r\n\r\n", host, host)

	br := bufio.NewReader(conn)
	status, err := br.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(status, "200") {
		t.Fatalf("CONNECT failed: %q", status)
	}
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if line == "\r\n" || line == "\n" {
			break
		}
	}
	return conn
}

func TestProxyMITMExtractsSession(t *testing.T) {
	// Upstream store: echoes a marker and records that it received the request.
	var upstreamSawToken string
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamSawToken = r.Header.Get("X-Token")
		_, _ = io.WriteString(w, "UPSTREAM_OK")
	}))
	defer upstream.Close()
	upstreamAddr := strings.TrimPrefix(upstream.URL, "https://")

	ca, err := NewCA()
	if err != nil {
		t.Fatal(err)
	}

	captured := make(chan *http.Request, 1)
	p := newProxy(ca, func(req *http.Request) { captured <- req })
	p.dialTLS = func(string) (net.Conn, error) {
		return tls.Dial("tcp", upstreamAddr, &tls.Config{InsecureSkipVerify: true, NextProtos: []string{"http/1.1"}})
	}

	proxyAddr, stop := startProxy(t, p)
	defer stop()

	conn := connectThroughProxy(t, proxyAddr, "buy.itunes.apple.com")
	defer func() { _ = conn.Close() }()

	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(ca.CertPEM())
	tlsConn := tls.Client(conn, &tls.Config{RootCAs: roots, ServerName: "buy.itunes.apple.com", NextProtos: []string{"http/1.1"}})
	if err := tlsConn.Handshake(); err != nil {
		t.Fatalf("client handshake: %v", err)
	}

	req, _ := http.NewRequest(http.MethodPost,
		"https://buy.itunes.apple.com/WebObjects/MZFinance.woa/wa/volumeStoreDownloadProduct?guid=603E5F8B2D32", nil)
	req.Header.Set("X-Token", "AwIAAAtoken==")
	req.Header.Set("X-Apple-Store-Front", "143478-2,34")
	req.Header.Set("iCloud-DSID", "1092660217")
	req.Header.Set("User-Agent", "Configurator/2.19")
	req.Header.Set("Cookie", "mz_at_ssl-1092660217=abc; itspod=42; mt-tkn-1092660217=xyz")
	req.Header.Set("Connection", "close")
	if err := req.Write(tlsConn); err != nil {
		t.Fatal(err)
	}

	resp, err := http.ReadResponse(bufio.NewReader(tlsConn), req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if string(body) != "UPSTREAM_OK" {
		t.Errorf("response body = %q, want UPSTREAM_OK (proxy must forward upstream)", body)
	}
	if upstreamSawToken != "AwIAAAtoken==" {
		t.Errorf("upstream X-Token = %q, want forwarded token", upstreamSawToken)
	}

	select {
	case got := <-captured:
		sess, err := charles.FromHTTPRequest(got)
		if err != nil {
			t.Fatalf("FromHTTPRequest: %v", err)
		}
		if sess.XToken != "AwIAAAtoken==" {
			t.Errorf("captured XToken = %q", sess.XToken)
		}
		if sess.GUID != "603E5F8B2D32" {
			t.Errorf("captured guid = %q", sess.GUID)
		}
		if sess.StoreFront != "143478-2,34" || sess.DSID != "1092660217" {
			t.Errorf("captured storefront/dsid wrong: %+v", sess)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("onRequest was never called for a MITM host")
	}
}

func TestProxyTunnelsPinnedHost(t *testing.T) {
	// Raw echo upstream stands in for a pinned host (e.g. gsa.apple.com).
	echoLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = echoLn.Close() }()
	go func() {
		for {
			c, err := echoLn.Accept()
			if err != nil {
				return
			}
			go func() { _, _ = io.Copy(c, c); _ = c.Close() }()
		}
	}()

	ca, _ := NewCA()
	onReqCalled := make(chan struct{}, 1)
	p := newProxy(ca, func(*http.Request) { onReqCalled <- struct{}{} })
	p.dialRaw = func(string) (net.Conn, error) { return net.Dial("tcp", echoLn.Addr().String()) }

	proxyAddr, stop := startProxy(t, p)
	defer stop()

	conn := connectThroughProxy(t, proxyAddr, "gsa.apple.com")
	defer func() { _ = conn.Close() }()

	// Raw bytes must pass through untouched (no TLS interception).
	want := "PINNED-RAW-BYTES"
	if _, err := io.WriteString(conn, want); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, len(want))
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatal(err)
	}
	if string(buf) != want {
		t.Errorf("tunnel echoed %q, want %q", buf, want)
	}

	select {
	case <-onReqCalled:
		t.Error("onRequest fired for a tunnelled (pinned) host — it must not be decrypted")
	case <-time.After(200 * time.Millisecond):
	}
}
