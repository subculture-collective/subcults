package indexer

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"testing"

	jetstream "github.com/bluesky-social/jetstream"
)

type scriptedV2Stream struct {
	items []scriptedV2Item
}

type scriptedV2Item struct {
	batch V2Batch
	err   error
}

func (s *scriptedV2Stream) Events(context.Context) iter.Seq2[V2Batch, error] {
	return func(yield func(V2Batch, error) bool) {
		for _, item := range s.items {
			if !yield(item.batch, item.err) {
				return
			}
		}
	}
}

func (s *scriptedV2Stream) Close() error { return nil }

type recordingV2Projector struct {
	cursors []uint64
	err     error
}

func (p *recordingV2Projector) Cursor(context.Context, ProjectionRequest) (uint64, error) {
	return 0, nil
}

func (p *recordingV2Projector) ApplyBatch(_ context.Context, _ ProjectionRequest, events []jetstream.Event, cursor uint64) (BatchResult, error) {
	if p.err != nil {
		return BatchResult{}, p.err
	}
	p.cursors = append(p.cursors, cursor)
	return BatchResult{Processed: int64(len(events))}, nil
}

func TestRunV2ConsumerContinuesAfterRecoverableStreamError(t *testing.T) {
	t.Parallel()
	stream := &scriptedV2Stream{items: []scriptedV2Item{
		{err: errors.New("temporary read")},
		{batch: V2Batch{Events: []jetstream.Event{{Seq: 8}}, Cursor: 8}},
	}}
	projector := &recordingV2Projector{}
	err := RunV2Consumer(context.Background(), stream, projector,
		ProjectionRequest{Consumer: "test", Target: ProjectionTargetActive}, nil, nil)
	if err != nil {
		t.Fatalf("RunV2Consumer(): %v", err)
	}
	if len(projector.cursors) != 1 || projector.cursors[0] != 8 {
		t.Fatalf("projected cursors = %v, want [8]", projector.cursors)
	}
}

func TestRunV2ConsumerStopsOnFatalStreamError(t *testing.T) {
	t.Parallel()
	stream := &scriptedV2Stream{items: []scriptedV2Item{{err: fmt.Errorf("%w: plan rejected", jetstream.ErrFatal)}}}
	err := RunV2Consumer(context.Background(), stream, &recordingV2Projector{},
		ProjectionRequest{Consumer: "test", Target: ProjectionTargetActive}, nil, nil)
	if !errors.Is(err, jetstream.ErrFatal) {
		t.Fatalf("RunV2Consumer() error = %v, want ErrFatal", err)
	}
}

func TestRunV2ConsumerStopsBeforeAdvancingAfterProjectionFailure(t *testing.T) {
	t.Parallel()
	stream := &scriptedV2Stream{items: []scriptedV2Item{
		{batch: V2Batch{Events: []jetstream.Event{{Seq: 3}}, Cursor: 3}},
		{batch: V2Batch{Events: []jetstream.Event{{Seq: 4}}, Cursor: 4}},
	}}
	projector := &recordingV2Projector{err: errors.New("database unavailable")}
	err := RunV2Consumer(context.Background(), stream, projector,
		ProjectionRequest{Consumer: "test", Target: ProjectionTargetActive}, nil, nil)
	if err == nil {
		t.Fatal("RunV2Consumer() succeeded, want projection failure")
	}
	if len(projector.cursors) != 0 {
		t.Fatalf("projected cursors = %v, want none", projector.cursors)
	}
}
