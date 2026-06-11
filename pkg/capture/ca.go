// Package capture runs a short-lived, selectively-intercepting HTTPS proxy so
// `istorepull capture` can extract a session straight from an Apple Configurator
// download — no manual Charles workflow. It decrypts only the store hosts and
// passes pinned hosts (gsa.apple.com) through untouched.
package capture

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"sync"
	"time"
)

// CA is an ephemeral certificate authority used to mint per-host leaf certs for
// the hosts we intercept. The private key never leaves the process.
type CA struct {
	cert    *x509.Certificate
	certDER []byte
	key     *ecdsa.PrivateKey

	leafKey *ecdsa.PrivateKey
	mu      sync.Mutex
	leaves  map[string]*tls.Certificate
}

// NewCA generates a fresh in-memory CA plus a leaf key shared across minted
// certificates.
func NewCA() (*CA, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate CA key: %w", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          serial(),
		Subject:               pkix.Name{CommonName: "iStorePull capture CA", Organization: []string{"iStorePull"}},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, fmt.Errorf("create CA cert: %w", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate leaf key: %w", err)
	}
	return &CA{
		cert:    cert,
		certDER: der,
		key:     key,
		leafKey: leafKey,
		leaves:  map[string]*tls.Certificate{},
	}, nil
}

// CertPEM returns the CA certificate in PEM form (for keychain trust).
func (c *CA) CertPEM() []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.certDER})
}

// leafFor mints (and caches) a leaf certificate for host, signed by the CA.
func (c *CA) leafFor(host string) (*tls.Certificate, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if tc, ok := c.leaves[host]; ok {
		return tc, nil
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial(),
		Subject:      pkix.Name{CommonName: host},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	if ip := net.ParseIP(host); ip != nil {
		tmpl.IPAddresses = []net.IP{ip}
	} else {
		tmpl.DNSNames = []string{host}
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, c.cert, &c.leafKey.PublicKey, c.key)
	if err != nil {
		return nil, fmt.Errorf("mint leaf for %s: %w", host, err)
	}
	tc := &tls.Certificate{
		Certificate: [][]byte{der, c.certDER},
		PrivateKey:  c.leafKey,
		Leaf:        mustParse(der),
	}
	c.leaves[host] = tc
	return tc, nil
}

// serverTLSConfig builds a TLS config that mints leaves per SNI and offers only
// HTTP/1.1 (so clients downgrade from h2, which our simple proxy can't speak).
func (c *CA) serverTLSConfig() *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		NextProtos: []string{"http/1.1"},
		GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			host := hello.ServerName
			if host == "" {
				host = "localhost"
			}
			return c.leafFor(host)
		},
	}
}

func serial() *big.Int {
	n, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		// crypto/rand failure is fatal-grade; fall back to a time-based serial.
		return big.NewInt(time.Now().UnixNano())
	}
	return n
}

func mustParse(der []byte) *x509.Certificate {
	cert, _ := x509.ParseCertificate(der)
	return cert
}
