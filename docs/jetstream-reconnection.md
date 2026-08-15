# Jetstream v2 replay and resume

The hand-written Jetstream v1 WebSocket reconnect client has been removed.
Subcults now uses the official Jetstream v2 Go SDK for archive negotiation,
download retries, replay-to-live cutover, live reconnects, and compression.

The indexer starts every subscription with `WithAfterSeq(lastCursor)`, including
`WithAfterSeq(0)` on a new database. The SDK replays sealed history and then
cuts over to live delivery. If a saved live position falls outside the live
lookback window, the SDK returns to archive replay instead of skipping the gap.

Subcults commits every SDK batch and `Batch.LastCursor()` in one PostgreSQL
transaction. The cursor is stored in `jetstream_v2_cursors`; the legacy
`indexer_state.cursor` timestamp remains untouched and is never read by v2.

See [the indexer package documentation](../internal/indexer/README.md) for event
folding, account deletion, reconciliation, and shadow rebuild behavior.
