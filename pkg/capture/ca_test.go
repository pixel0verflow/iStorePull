package capture

import (
	"crypto/x509"
	"testing"
)

func TestCALeafChainsAndMatchesHost(t *testing.T) {
	ca, err := NewCA()
	if err != nil {
		t.Fatal(err)
	}

	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(ca.CertPEM()) {
		t.Fatal("CertPEM did not parse as a root")
	}

	leaf, err := ca.leafFor("buy.itunes.apple.com")
	if err != nil {
		t.Fatal(err)
	}
	if leaf.Leaf == nil {
		t.Fatal("leaf.Leaf not populated")
	}

	if _, err := leaf.Leaf.Verify(x509.VerifyOptions{
		Roots:   roots,
		DNSName: "buy.itunes.apple.com",
	}); err != nil {
		t.Fatalf("leaf does not verify against CA for host: %v", err)
	}

	// wrong host must fail
	if _, err := leaf.Leaf.Verify(x509.VerifyOptions{
		Roots:   roots,
		DNSName: "gsa.apple.com",
	}); err == nil {
		t.Error("leaf wrongly verified for a different host")
	}
}

func TestCALeafCached(t *testing.T) {
	ca, err := NewCA()
	if err != nil {
		t.Fatal(err)
	}
	a, _ := ca.leafFor("buy.itunes.apple.com")
	b, _ := ca.leafFor("buy.itunes.apple.com")
	if a != b {
		t.Error("leafFor should cache and return the same cert per host")
	}
}

func TestServerTLSConfigHTTP1Only(t *testing.T) {
	ca, _ := NewCA()
	cfg := ca.serverTLSConfig()
	if len(cfg.NextProtos) != 1 || cfg.NextProtos[0] != "http/1.1" {
		t.Errorf("NextProtos = %v, want [http/1.1] (force h2 downgrade)", cfg.NextProtos)
	}
}
