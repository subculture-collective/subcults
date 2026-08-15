package indexer

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"time"

	jetstream "github.com/bluesky-social/jetstream"
)

// V2Batch is the transport-independent batch shape consumed by the indexer and
// backfill command. Events are valid until the stream advances to its next SDK
// batch and therefore must be projected synchronously.
type V2Batch struct {
	Events []jetstream.Event
	Cursor uint64
}

// V2Stream exposes ordered Jetstream v2 batches.
type V2Stream interface {
	Events(ctx context.Context) iter.Seq2[V2Batch, error]
	Close() error
}

// V2Subscription configures the official Jetstream v2 SDK.
type V2Subscription struct {
	Host         string
	APIKey       string
	Collections  []string
	AfterSeq     uint64
	Replay       bool
	BeforeSeq    *uint64
	SnapshotOnly bool
	BatchSize    int
	Logger       *slog.Logger
}

type sdkV2Stream struct {
	client *jetstream.Client
}

// SubscribeV2 opens an official Jetstream v2 subscription.
func SubscribeV2(config V2Subscription) (V2Stream, error) {
	if config.Host == "" {
		return nil, errors.New("jetstream v2 host is required")
	}
	options := make([]jetstream.Option, 0, 8)
	if len(config.Collections) > 0 {
		options = append(options, jetstream.WithCollections(config.Collections))
	}
	if config.Replay {
		options = append(options, jetstream.WithAfterSeq(config.AfterSeq))
	}
	if config.BeforeSeq != nil {
		options = append(options, jetstream.WithBeforeSeq(*config.BeforeSeq))
	}
	if config.SnapshotOnly {
		options = append(options, jetstream.WithSnapshotOnly())
	}
	if config.BatchSize > 0 {
		options = append(options, jetstream.WithBatchSize(config.BatchSize))
	}
	if config.APIKey != "" {
		options = append(options, jetstream.WithAPIKey(config.APIKey))
	}
	if config.Logger != nil {
		options = append(options, jetstream.WithLogger(config.Logger))
	}
	client, err := jetstream.Subscribe(config.Host, options...)
	if err != nil {
		return nil, fmt.Errorf("subscribe to Jetstream v2: %w", err)
	}
	return &sdkV2Stream{client: client}, nil
}

func (s *sdkV2Stream) Events(ctx context.Context) iter.Seq2[V2Batch, error] {
	return func(yield func(V2Batch, error) bool) {
		for batch, err := range s.client.Events(ctx) {
			if err != nil {
				if !yield(V2Batch{}, err) {
					return
				}
				continue
			}
			if batch == nil {
				if !yield(V2Batch{}, errors.New("jetstream v2 yielded a nil batch")) {
					return
				}
				continue
			}
			if !yield(V2Batch{Events: batch.Events(), Cursor: batch.LastCursor()}, nil) {
				return
			}
		}
	}
}

func (s *sdkV2Stream) Close() error {
	return s.client.Close()
}

// RunV2Consumer projects replay and live batches until cancellation or a fatal
// stream/database error. A database error stops consumption so the last
// committed cursor remains the exact resume point.
func RunV2Consumer(ctx context.Context, stream V2Stream, projector V2Projector, request ProjectionRequest, logger *slog.Logger, metrics *Metrics) error {
	if stream == nil {
		return errors.New("jetstream v2 stream is required")
	}
	if projector == nil {
		return errors.New("jetstream v2 projector is required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	for batch, streamErr := range stream.Events(ctx) {
		if streamErr != nil {
			if errors.Is(streamErr, jetstream.ErrFatal) {
				return fmt.Errorf("fatal Jetstream v2 stream failure: %w", streamErr)
			}
			logger.Warn("recoverable Jetstream v2 stream error", slog.String("error", streamErr.Error()))
			continue
		}
		started := time.Now()
		result, err := projector.ApplyBatch(ctx, request, batch.Events, batch.Cursor)
		if err != nil {
			if metrics != nil {
				metrics.IncDatabaseWritesFailed()
			}
			return fmt.Errorf("project Jetstream v2 batch ending at %d: %w", batch.Cursor, err)
		}
		if metrics != nil {
			for range result.Processed {
				metrics.IncMessagesProcessed()
			}
			for range result.Quarantined {
				metrics.IncMessagesError()
			}
			metrics.ObserveIngestLatency(time.Since(started).Seconds())
			if len(batch.Events) > 0 {
				last := batch.Events[len(batch.Events)-1]
				if last.TimeUS > 0 {
					metrics.SetProcessingLag(time.Since(time.UnixMicro(last.TimeUS)).Seconds())
				}
			}
		}
		logger.Debug("committed Jetstream v2 batch",
			slog.Uint64("cursor", batch.Cursor),
			slog.Int64("processed", result.Processed),
			slog.Int64("skipped", result.Skipped),
			slog.Int64("quarantined", result.Quarantined),
			slog.Int64("deleted", result.Deleted))
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return nil
}
