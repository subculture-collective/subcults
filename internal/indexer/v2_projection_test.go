package indexer

import (
	"testing"

	jetstream "github.com/bluesky-social/jetstream"
)

func TestValidateProjectionRequest(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		request ProjectionRequest
		wantErr bool
	}{
		{name: "active", request: ProjectionRequest{Consumer: "live", Target: ProjectionTargetActive}},
		{name: "shadow", request: ProjectionRequest{Consumer: "replay", Target: ProjectionTargetShadow, RebuildID: "release"}},
		{name: "missing consumer", request: ProjectionRequest{Target: ProjectionTargetActive}, wantErr: true},
		{name: "missing rebuild", request: ProjectionRequest{Consumer: "replay", Target: ProjectionTargetShadow}, wantErr: true},
		{name: "active rebuild", request: ProjectionRequest{Consumer: "live", Target: ProjectionTargetActive, RebuildID: "wrong"}, wantErr: true},
		{name: "unknown target", request: ProjectionRequest{Consumer: "live", Target: "other"}, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := validateProjectionRequest(test.request)
			if (err != nil) != test.wantErr {
				t.Fatalf("validateProjectionRequest() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestValidateBatch(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		events  []jetstream.Event
		cursor  uint64
		wantErr bool
	}{
		{name: "ordered", events: []jetstream.Event{{Seq: 4}, {Seq: 5}}, cursor: 5},
		{name: "empty", cursor: 0},
		{name: "zero event", events: []jetstream.Event{{Seq: 0}}, cursor: 0, wantErr: true},
		{name: "out of order", events: []jetstream.Event{{Seq: 5}, {Seq: 4}}, cursor: 4, wantErr: true},
		{name: "duplicate sequence", events: []jetstream.Event{{Seq: 5}, {Seq: 5}}, cursor: 5, wantErr: true},
		{name: "cursor behind", events: []jetstream.Event{{Seq: 5}}, cursor: 4, wantErr: true},
		{name: "cursor ahead", events: []jetstream.Event{{Seq: 5}}, cursor: 6, wantErr: true},
		{name: "empty nonzero", cursor: 1, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := validateBatch(test.events, test.cursor)
			if (err != nil) != test.wantErr {
				t.Fatalf("validateBatch() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestValidateOnlyRejectsMalformedEventShapes(t *testing.T) {
	t.Parallel()
	projector := NewPostgresV2Projector(nil, NewRecordFilter(NewFilterMetrics()), nil)
	tests := []struct {
		name  string
		event jetstream.Event
	}{
		{name: "missing commit", event: jetstream.Event{Seq: 1, Kind: jetstream.KindCommit}},
		{name: "missing identity", event: jetstream.Event{Seq: 1, Kind: jetstream.KindIdentity}},
		{name: "unknown kind", event: jetstream.Event{Seq: 1, Kind: jetstream.Kind("unknown")}},
		{name: "unknown operation", event: jetstream.Event{Seq: 1, Kind: jetstream.KindCommit,
			Commit: &jetstream.Commit{Operation: jetstream.Operation("replace"), Collection: CollectionScene}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := projector.validateOnly([]jetstream.Event{test.event}); err == nil {
				t.Fatal("validateOnly() succeeded, want malformed event error")
			}
		})
	}
}
