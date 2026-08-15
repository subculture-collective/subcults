package backfill

import (
	"context"
	"fmt"
	"io"
	"iter"
	"log/slog"
	"sync"

	jetstream "github.com/bluesky-social/jetstream"
	"github.com/onnwee/subcults/internal/indexer"
)

// InMemoryCheckpointStore implements CheckpointStore for testing.
type InMemoryCheckpointStore struct {
	mu          sync.Mutex
	checkpoints map[int64]*Checkpoint
	nextID      int64
}

func newInMemoryCheckpointStore() *InMemoryCheckpointStore {
	return &InMemoryCheckpointStore{
		checkpoints: make(map[int64]*Checkpoint),
	}
}

func (s *InMemoryCheckpointStore) GetLatest(ctx context.Context, source string, target indexer.ProjectionTarget, rebuildID string) (*Checkpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var latest *Checkpoint
	for _, cp := range s.checkpoints {
		if cp.Source == source && cp.Target == target && cp.RebuildID == rebuildID {
			if latest == nil || cp.ID > latest.ID {
				latest = cp
			}
		}
	}
	return latest, nil
}

func (s *InMemoryCheckpointStore) Create(ctx context.Context, source string, target indexer.ProjectionTarget, rebuildID string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	s.checkpoints[s.nextID] = &Checkpoint{
		ID:        s.nextID,
		Source:    source,
		Target:    target,
		RebuildID: rebuildID,
		Status:    "running",
	}
	return s.nextID, nil
}

func (s *InMemoryCheckpointStore) Update(ctx context.Context, cp *Checkpoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, ok := s.checkpoints[cp.ID]
	if !ok {
		return fmt.Errorf("checkpoint %d not found", cp.ID)
	}
	existing.CursorTS = cp.CursorTS
	existing.CursorSeq = cp.CursorSeq
	existing.CAROffset = cp.CAROffset
	existing.RecordsProcessed = cp.RecordsProcessed
	existing.RecordsSkipped = cp.RecordsSkipped
	existing.ErrorsCount = cp.ErrorsCount
	return nil
}

func (s *InMemoryCheckpointStore) Complete(ctx context.Context, id int64, processed, skipped, errors int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp, ok := s.checkpoints[id]
	if !ok {
		return fmt.Errorf("checkpoint %d not found", id)
	}
	cp.Status = "completed"
	cp.RecordsProcessed = processed
	cp.RecordsSkipped = skipped
	cp.ErrorsCount = errors
	return nil
}

func (s *InMemoryCheckpointStore) Fail(ctx context.Context, id int64, processed, skipped, errors int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp, ok := s.checkpoints[id]
	if !ok {
		return fmt.Errorf("checkpoint %d not found", id)
	}
	cp.Status = "failed"
	cp.RecordsProcessed = processed
	cp.RecordsSkipped = skipped
	cp.ErrorsCount = errors
	return nil
}

func (s *InMemoryCheckpointStore) get(id int64) *Checkpoint {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.checkpoints[id]
}

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newTestFilter() *indexer.RecordFilter {
	return indexer.NewRecordFilter(indexer.NewFilterMetrics())
}

func newTestRepo() *indexer.InMemoryRecordRepository {
	return indexer.NewInMemoryRecordRepository(newTestLogger())
}

type fakeV2Stream struct {
	batches []indexer.V2Batch
	errors  []error
}

func (s *fakeV2Stream) Events(context.Context) iter.Seq2[indexer.V2Batch, error] {
	return func(yield func(indexer.V2Batch, error) bool) {
		for _, err := range s.errors {
			if !yield(indexer.V2Batch{}, err) {
				return
			}
		}
		for _, batch := range s.batches {
			if !yield(batch, nil) {
				return
			}
		}
	}
}

func (s *fakeV2Stream) Close() error { return nil }

type fakeV2Projector struct {
	cursor  uint64
	results []indexer.BatchResult
	err     error
}

func (p *fakeV2Projector) Cursor(context.Context, indexer.ProjectionRequest) (uint64, error) {
	return p.cursor, nil
}

func (p *fakeV2Projector) ApplyBatch(_ context.Context, _ indexer.ProjectionRequest, events []jetstream.Event, cursor uint64) (indexer.BatchResult, error) {
	if p.err != nil {
		return indexer.BatchResult{}, p.err
	}
	p.cursor = cursor
	if len(p.results) > 0 {
		result := p.results[0]
		p.results = p.results[1:]
		return result, nil
	}
	return indexer.BatchResult{Processed: int64(len(events))}, nil
}
