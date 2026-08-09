package atprotocol

import "testing"

func TestSafeReturnPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/settings", true},
		{"/studio/events?published=1", true},
		{"https://evil.example", false},
		{"//evil.example", false},
		{"/settings\nLocation:https://evil.example", false},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			if got := safeReturnPath(test.path); got != test.want {
				t.Fatalf("safeReturnPath(%q) = %v, want %v", test.path, got, test.want)
			}
		})
	}
}
