# Canonical AT Protocol Records

Subcults publishes disclosure-safe public content to creator-controlled AT
Protocol repositories under the owned `tv.subcult.*` namespace. PostgreSQL is
the system of record for private drafts, local authorization, contact and
consent data, delivery history, protected locations, payments, and public
discovery projections.

The executable lexicons are the versioned JSON files in `lexicons/tv/subcult/`:

| Collection | Portable meaning |
| --- | --- |
| `tv.subcult.profile` | Public presentation identity |
| `tv.subcult.act` | Creative project linked to a Profile |
| `tv.subcult.place` | Coarse public city or region context |
| `tv.subcult.venue` | Named venue linked to a Place, never a private address |
| `tv.subcult.scene` | Community or publication context |
| `tv.subcult.event` | One place-and-time occurrence |
| `tv.subcult.tour` | Itinerary grouping for Appearances |
| `tv.subcult.appearance` | One Act's participation in an Event |
| `tv.subcult.assertion` | Append-only, source-attributed correction claim |

Relationships use AT URIs and strong references. Local database UUIDs are not
portable identifiers and must never appear in emitted records. Records require
their canonical `$type`, RFC 3339 timestamps, collection-specific fields, and
disclosure validation before a PDS write.

## Location boundary

Portable Place and Venue records may contain only public/coarse data. The
validator recursively rejects precise coordinates, latitude/longitude, street
addresses, access instructions, encrypted values, and protected-location
grants. Exact protected venue data must not enter an AT record, sync payload
log, projection cache, or public API response.

An Event points to where it occurs. An Act home territory is a separately
declared public association; it is not an inferred current location. An
Appearance connects an Act to an Event and may optionally point to a Tour.

## Assertions and corrections

Assertions preserve provider/source URL, observation and effective timestamps,
confidence, correction state, subject record, and the assertion being
superseded. They are appended rather than rewritten. Event cancellation and
postponement update the canonical Event record; true deletion uses repository
deletion and archives the local projection.

## Namespace transition

New writes use only `tv.subcult.*`. The synchronizer observes legacy
`app.subcult.*` records without republishing them during the time-bounded
compatibility window. Legacy observation does not establish a new public
projection without an explicit migration mapping. Retirement requires an
inventory proving no dependent records remain.

## Validation

```bash
./scripts/lint-lexicons.sh
go test ./internal/atprotocol ./internal/indexer -count=1
```

The lint script uses the pinned official Indigo lexicon generator. Runtime
validation remains mandatory because PDS acceptance alone does not enforce
Subcults' privacy policy or relationship semantics.

See [AT Protocol and PDS Operations](operations/ATPROTO_PDS.md) and
[ADR-008](adr/0008-atproto-canonical-public-data.md).
