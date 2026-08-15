# Jetstream v2 indexer

Subcults consumes AT Protocol events through the official
`github.com/bluesky-social/jetstream` v2 Go SDK. The SDK owns archive planning,
download retries, replay-to-live cutover, compression, and transport resume.
Subcults owns projection semantics and database atomicity.

## Cursor contract

Jetstream v2 `Event.Seq` is the cursor. It is not `Event.TimeUS` and is not the
legacy v1 `indexer_state.cursor` value.

Durable v2 cursors live in `jetstream_v2_cursors`, keyed by consumer, target,
and rebuild ID. `PostgresV2Projector.ApplyBatch` folds every event in an SDK
batch and updates `Batch.LastCursor()` in the same PostgreSQL transaction. If
any database operation fails, both the event changes and cursor roll back.

Normal operation always supplies `WithAfterSeq(lastCursor)`. This lets the SDK
replay sealed history after downtime and cut over to the live tail without a
consumer-visible gap. The initial cursor is zero.

## Folding semantics

- Commit creates and updates are idempotent by canonical AT URI plus revision.
- Commit deletes suppress the corresponding local record.
- Identity events update `jetstream_v2_identities`.
- Inactive account events suppress every indexed record owned by the DID.
- Reactivated accounts and sync-divergence events enqueue a targeted durable
  reconciliation and emit PostgreSQL `NOTIFY jetstream_reconciliation` after
  the transaction commits.
- Invalid or conflicting canonical records are quarantined without blocking
  later events; database/transaction failures stop the consumer before cursor
  advancement.

Delivery is at least once. All projection operations therefore remain safe to
repeat.

## Backfill and rebuilds

`subcults-backfill` uses the same official event shape and projector. Its
Jetstream contract is sequence based:

```text
backfill --source=jetstream --target=shadow \
  --rebuild-id=release-2026-08 --after-seq=0
```

`--before-seq` adds an inclusive upper bound. Jetstream mode is snapshot-only,
so it stops at the sealed archive range and never follows live data.

Shadow rebuilds fold into `jetstream_v2_shadow_records` and rebuild-namespaced
account, identity, reconciliation, and quarantine tables; they do not mutate
product or live v2 state tables. Query `jetstream_v2_projection_comparison` for
current count, shadow count, and one-sided AT URI differences before approving
any separately implemented promotion. `time_us` remains available in the
shadow table for bounded analysis, but never acts as a resume cursor.

## Configuration

```dotenv
DATABASE_URL=postgres://...
JETSTREAM_HOST=jetstream.us-west.bsky.network
JETSTREAM_API_KEY=
JETSTREAM_BATCH_SIZE=256
```

`JETSTREAM_API_KEY` is optional for public archives. The SDK sends it only to
archive negotiation and download endpoints, not the live WebSocket.
