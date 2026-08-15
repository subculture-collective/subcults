# Jetstream v2 backfill and rebuild plan

`subcults-backfill` consumes the same official Jetstream v2 event batches as
the live indexer. Jetstream mode is a sealed-archive snapshot: it never cuts
over to the live tail and always uses sequence bounds.

## Safe default: shadow rebuild

```bash
./bin/backfill \
  --source=jetstream \
  --target=shadow \
  --rebuild-id=release-2026-08 \
  --after-seq=0
```

The rebuild folds every supported record mutation into
`jetstream_v2_shadow_records`, keyed by canonical AT URI. Account deletion and
takedown markers suppress all records for that DID. Identity and sync events
are processed explicitly, and sync divergence creates a durable targeted
reconciliation request. Shadow account, identity, reconciliation, and
quarantine state is namespaced by rebuild ID and never enters the live v2
state tables.

The shadow table never changes product-facing scene, event, post, alliance, or
canonical publication tables. Query `jetstream_v2_projection_comparison` after
completion to compare active and replay-derived record counts and one-sided AT
URI differences.

## Sequence ranges

`--after-seq` is exclusive. `--before-seq` is inclusive and optional:

```bash
./bin/backfill \
  --source=jetstream \
  --target=shadow \
  --rebuild-id=bounded-analysis \
  --after-seq=1000000 \
  --before-seq=2000000
```

`time_us` is retained in the shadow projection for time-window analysis, but
it is never a resume cursor. Translating timestamps to v2 sequences requires a
separately validated lookup process and is intentionally not part of this CLI.

## Resume and atomicity

Each SDK batch and `Batch.LastCursor()` commit in one PostgreSQL transaction.
The separately visible `backfill_checkpoints.cursor_seq` follows the projection
commit. On restart, `--resume=true` chooses the greatest of the requested lower
bound, the projection cursor, and the matching checkpoint cursor. A database
failure stops the run before the cursor can advance.

Rebuild IDs are stable operator-selected namespaces. Reuse the same ID to
resume an interrupted rebuild; use a new ID to produce an independent replay.

## Active projection backfill

`--target=active` is available for an explicitly approved repair:

```bash
./bin/backfill --source=jetstream --target=active --after-seq=123456
```

This folds directly into product tables. Use it only with a database rollback
boundary and after reviewing the requested sequence range. AT URI plus revision
idempotency makes repeated delivery safe, but it does not replace operational
approval for active-table mutation.

## Dry run and CAR input

`--dry-run` validates delivered records without changing projection tables or
projection cursors. It also leaves backfill checkpoints unchanged, so a later
real run with the same rebuild ID cannot accidentally resume past dry-run data.

CAR imports remain a separate source contract:

```bash
./bin/backfill --source=car --car-file=/data/exports/repo.car
```

CAR mode does not accept Jetstream sequence or shadow-target options.
