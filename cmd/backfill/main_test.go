package main

import "testing"

func TestValidateFlags(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		source    string
		target    string
		rebuildID string
		carPath   string
		after     uint64
		before    uint64
		batch     int
		wantErr   bool
	}{
		{name: "shadow snapshot", source: "jetstream", target: "shadow", rebuildID: "release", before: 20, batch: 100},
		{name: "active snapshot", source: "jetstream", target: "active", batch: 100},
		{name: "car", source: "car", target: "shadow", carPath: "export.car", batch: 100},
		{name: "shadow needs rebuild ID", source: "jetstream", target: "shadow", batch: 100, wantErr: true},
		{name: "active rejects rebuild ID", source: "jetstream", target: "active", rebuildID: "wrong", batch: 100, wantErr: true},
		{name: "sequence bounds", source: "jetstream", target: "active", after: 20, before: 20, batch: 100, wantErr: true},
		{name: "positive batch", source: "jetstream", target: "active", batch: 0, wantErr: true},
		{name: "car needs path", source: "car", target: "shadow", batch: 100, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := validateFlags(test.source, test.target, test.rebuildID, test.carPath, test.after, test.before, test.batch)
			if (err != nil) != test.wantErr {
				t.Fatalf("validateFlags() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}
