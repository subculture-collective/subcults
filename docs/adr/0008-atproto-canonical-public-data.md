# ADR-008: Creator PDS Records Are Canonical Public Data

- Status: Accepted
- Date: 2026-08-09

## Context

Subcults needs portable creator-owned public records without putting drafts,
consent, contacts, payments, or protected locations onto a public protocol.
The prior ingestion-only `app.subcult.*` model made PostgreSQL canonical and
used an uncontrolled namespace.

## Decision

Use creator PDS repositories and owned `tv.subcult.*` lexicons as the canonical
public layer. Keep PostgreSQL canonical for private/local data and as a
validated public discovery projection. Publish through confidential AT
Protocol OAuth and promote projections only after Tap or authoritative-PDS
reconciliation validates the exact record CID. Continue passwordless email as
the local recovery and authorization identity; external DID linking is
one-to-one and additive.

## Consequences

- PDS writes can succeed while discovery projection is temporarily delayed.
- Portable relationships require AT URIs, creating an explicit publication
  order for Profiles/Acts, Places/Venues, Events, Tours, and Appearances.
- PostgreSQL must retain projection checkpoints, digest-only observations,
  failure quarantine, mapping, OAuth, and provisioning audit state.
- Public provisioning depends on PDS operational capacity and enforceable
  invitation expiry, so it is gated separately from linking and publishing.
- ADR-003 remains historical for Jetstream delivery, but Tap becomes the
  authoritative tracker/backfill mechanism after the shadow parity gate.
