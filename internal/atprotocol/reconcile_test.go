package atprotocol

import (
	"net"
	"net/url"
	"testing"
)

func TestValidateAuthoritativePDSURLRejectsUnsafeOrigins(t *testing.T) {
	t.Parallel()
	tests := []string{
		"http://pds.example.com",
		"https://localhost",
		"https://127.0.0.1",
		"https://[::1]",
		"https://10.0.0.2",
		"https://169.254.169.254",
		"https://user:secret@pds.example.com",
	}
	for _, raw := range tests {
		raw := raw
		t.Run(raw, func(t *testing.T) {
			t.Parallel()
			parsed, err := url.Parse(raw)
			if err != nil {
				t.Fatalf("parse test URL: %v", err)
			}
			if err := validateAuthoritativePDSURL(parsed); err == nil {
				t.Fatalf("expected %q to be rejected", raw)
			}
		})
	}
}

func TestValidateAuthoritativePDSURLAllowsPublicHTTPSOrigin(t *testing.T) {
	t.Parallel()
	parsed, err := url.Parse("https://pds.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if err := validateAuthoritativePDSURL(parsed); err != nil {
		t.Fatalf("expected public HTTPS origin to pass: %v", err)
	}
}

func TestPublicInternetIPClassification(t *testing.T) {
	t.Parallel()
	if !isPublicInternetIP(net.ParseIP("203.0.113.1")) {
		t.Fatal("expected documentation public address to pass classification")
	}
	if isPublicInternetIP(net.ParseIP("192.168.1.2")) {
		t.Fatal("expected private address to fail classification")
	}
}
