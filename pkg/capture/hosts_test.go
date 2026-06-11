package capture

import "testing"

func TestShouldMITM(t *testing.T) {
	mitm := []string{
		"buy.itunes.apple.com",
		"buy.itunes.apple.com:443",
		"downloaddispatch.itunes.apple.com",
		"p25-buy.itunes.apple.com",
		"P71-BUY.ITUNES.APPLE.COM",
	}
	tunnel := []string{
		"gsa.apple.com",
		"init.itunes.apple.com",
		"apple.com",
		"example.com",
		"buy.itunes.apple.com.evil.com",
	}
	for _, h := range mitm {
		if !shouldMITM(h) {
			t.Errorf("shouldMITM(%q) = false, want true", h)
		}
	}
	for _, h := range tunnel {
		if shouldMITM(h) {
			t.Errorf("shouldMITM(%q) = true, want false (must be tunnelled)", h)
		}
	}
}
