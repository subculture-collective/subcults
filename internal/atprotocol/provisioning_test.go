package atprotocol

import "testing"

func TestNormalizeProvisionedHandle(t *testing.T) {
	tests := []struct {
		input string
		want  string
		ok    bool
	}{
		{"night-signal", "night-signal.subcult.tv", true},
		{"Night-Signal.subcult.tv", "night-signal.subcult.tv", true},
		{"admin", "", false},
		{"x", "", false},
		{"bad_name", "", false},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			got, err := normalizeProvisionedHandle(test.input, "subcult.tv")
			if (err == nil) != test.ok || got != test.want {
				t.Fatalf("got %q, %v; want %q, ok=%v", got, err, test.want, test.ok)
			}
		})
	}
}

func TestCanonicalIPDoesNotTrustForwardedChain(t *testing.T) {
	if got := canonicalIP("203.0.113.8, 10.0.0.2"); got != "203.0.113.8" {
		t.Fatalf("canonical IP = %q", got)
	}
}
