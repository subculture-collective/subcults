# Domain Glossary

Shared vocabulary for the Subcults codebase. Each entry includes the canonical definition, where it's defined, and related terms. Reference this glossary on first use of a domain term in any document.

See [ARCHITECTURE.md](ARCHITECTURE.md) for system-level context.

---

## Core Entities

### Scene

An underground music community or venue with privacy-conscious location. The primary unit of discovery on the platform.

- **Defined in:** `internal/scene/model.go` — `Scene` struct
- **Key fields:** `ID`, `DID`, `Name`, `Description`, `PrecisePoint`, `CoarseGeohash`, `AllowPrecise`
- **Storage:** `scenes` table with PostGIS `GEOGRAPHY(Point, 4326)` column
- **Privacy:** Location consent enforced via `EnforceLocationConsent()` — clears `PrecisePoint` if `AllowPrecise` is false
- **Related:** Event, Post, Membership, Alliance

### Profile

A DID-controlled public identity for an artist, venue, festival, promoter,
collective, label, or curator. A Profile can be affiliated with Scenes without
being identical to a Scene.

- **Defined in:** `internal/touring/model.go` and migration `000033`
- **Related:** Act, Scene, Home Territory

### Act

A public creative project that can be billed in an Event Appearance. An Act is
not necessarily one user and does not become part of a destination Scene merely
by performing there.

- **Defined in:** `internal/touring/model.go` and migration `000033`
- **Related:** Profile, Appearance, Tour

### Place

A canonical city, market, or region and timezone used for geographic discovery.
A Place is not a user's current location.

- **Defined in:** `internal/touring/model.go` and migration `000033`
- **Related:** Venue, Event, Home Territory

### Venue

A named hosting location within a Place. Venue retention and disclosure rules
are independent of Scene and Act home-location privacy.

- **Defined in:** `internal/touring/model.go` and migration `000033`
- **Related:** Place, Event, Allow Precise

### Home Territory

A coarse, declared, temporal affinity between an Act and a Place. It is cultural
context, not a residence or live-location inference.

- **Defined in:** `internal/touring/model.go` and migration `000033`
- **Related:** Act, Place, Appearance

### Event

A time-specific occurrence with its own Place/location, host context, and
optional ticket/capacity support. During compatibility migration every Event
retains a primary host `SceneID`; that field does not represent an Act's home
base.

- **Defined in:** `internal/scene/model.go` — `Event` struct
- **Key fields:** `ID`, `SceneID`, `Title`, `StartsAt`, `EndsAt`, `PrecisePoint`, `AllowPrecise`
- **Storage:** `events` table with same PostGIS/geohash pattern as Scene
- **Related:** Scene, Place, Venue, Appearance, Stream

### Appearance

An Act's billed participation in an Event. A tour stop is a tour-linked
Appearance; a festival appearance points to a festival Event; a one-off has no
Tour.

- **Defined in:** `internal/touring/model.go`, `internal/touring/repository.go`, and migration `000034`
- **Related:** Act, Event, Tour

### Tour

A named grouping of Appearances for a primary Act. A Tour does not own or
override the occurrence location of its Events.

- **Defined in:** `internal/touring/model.go`, `internal/touring/repository.go`, and migration `000034`
- **Related:** Act, Appearance, Event

### Signal

A versioned, time-bound invitation to take an action related to a Scene, Profile,
Event, Appearance, Tour, Post, Stream, or commerce offer. Signal is the
provisional participant-facing term for the campaign bounded context.

- **Defined in:** `internal/signal/model.go`, `internal/signal/service.go`, and migration `000036`
- **Related:** Audience Relationship, Consent Scope, Suppression

### Audience Relationship

A source-attributed connection between a participant/contact and a Scene or
Profile. Membership, RSVP, purchase, attendance, Stream participation, interest,
and delivery consent remain distinct evidence types.

- **Defined in:** `internal/audience/model.go`, `internal/audience/service.go`, and migration `000035`
- **Related:** Signal, Consent Scope, Membership

### Consent Scope

A sender/program, channel, purpose, disclosure version, and optional touring or
place boundary against which append-only grant and revocation events are
recorded. Contact verification is evidence on the Contact Point or DID link,
not consent. Suppression is evaluated from its own enforcement ledger.

- **Defined in:** `internal/audience/model.go`, `internal/audience/service.go`, and migration `000035`
- **Related:** Signal, Audience Relationship, Suppression

### Suppression

An enforcement record that blocks delivery globally or for a channel, sender,
or Consent Scope. Applicable suppression always overrides a consent grant.

- **Defined in:** `internal/audience/model.go`, `internal/audience/service.go`, and migration `000035`
- **Related:** Consent Scope, Signal

### Post

Text content with optional media attachments, linked to a Scene or Event. Ingested from AT Protocol records.

- **Defined in:** `internal/scene/post.go` — `Post` struct
- **Key fields:** `ID`, `SceneID`, `AuthorDID`, `Content`, `Attachments`
- **Related:** Scene, AT Protocol, Jetstream

### Stream

A live audio session hosted in a LiveKit room, associated with a Scene.

- **Defined in:** `internal/scene/stream.go` — `Stream` struct
- **Key fields:** `ID`, `SceneID`, `RoomName`, `HostDID`, `Status`, `StartedAt`, `EndedAt`
- **Lifecycle:** `pending` → `live` → `ended`
- **Related:** Participant, LiveKit Token

### Alliance

A directional trust relationship between two Scenes. Scene A endorsing Scene B means A vouches for B's legitimacy.

- **Defined in:** `internal/trust/types.go` — `Alliance` struct
- **Key fields:** `SourceSceneID`, `TargetSceneID`, `Weight` (0.0–1.0), `Status` (active/revoked)
- **Scoring:** Only `active` alliances contribute to trust computation
- **Related:** Trust Score, Effective Weight

### Membership

A user's participation in a Scene with a specific role and trust weight.

- **Defined in:** `internal/trust/types.go` — `Membership` struct
- **Key fields:** `SceneID`, `UserDID`, `Role`, `TrustWeight` (0.0–1.0)
- **Roles:** `owner` (1.0×), `curator` (0.8×), `member` (0.5×), `guest` (0.3×)
- **Related:** Role Multiplier, Effective Weight

---

## Trust & Ranking

### Trust Score

Composite score (0.0–1.0) representing a Scene's community-validated credibility. Used in search ranking when the `trust_ranking` feature flag is enabled.

- **Formula:** `avg(alliance_weights) × avg(effective_membership_weights)`
- **Computed by:** `internal/trust/calculator.go` — `ComputeSceneTrust()`
- **Search integration:** `composite_score = (text_relevance × 0.4) + (proximity × 0.3) + (recency × 0.2) + (trust_weight × 0.1)`
- **Related:** Alliance, Effective Weight

### Effective Weight

How much a single Membership contributes to a Scene's trust score.

- **Formula:** `TrustWeight × RoleMultiplier`
- **Example:** A curator (0.8×) with trust weight 0.7 → effective weight 0.56
- **Related:** Role Multiplier, Trust Score

### Role Multiplier

Fixed multiplier per membership role, reflecting influence on Scene trust.

| Role    | Multiplier |
| ------- | ---------- |
| Owner   | 1.0        |
| Curator | 0.8        |
| Member  | 0.5        |
| Guest   | 0.3        |

- **Defined in:** `internal/trust/calculator.go`

---

## Geographic & Privacy

### Jitter

Deterministic random offset applied to coordinates for map display, preventing precise location tracking of scenes that haven't opted in.

- **Defined in:** `internal/geo/jitter.go` — `ApplyJitter()`
- **Behavior:** ~250m default displacement, deterministic (same input → same output)
- **Purpose:** Public map markers show approximate, not exact, locations
- **Related:** Allow Precise, Coarse Geohash

### Coarse Geohash

A 6-character base32 string encoding approximate location (~±0.61 km accuracy) for discovery without revealing precise coordinates.

- **Defined in:** `internal/geo/geohash.go`
- **Storage:** `coarse_geohash VARCHAR(20)` column on scenes/events tables
- **Usage:** Enables area-based search without exposing exact position
- **Related:** Jitter, Precise Point

### Precise Point

Exact latitude/longitude stored as PostGIS `GEOGRAPHY(Point, 4326)`. Only persisted when a Scene's owner has explicitly granted consent.

- **Storage:** `precise_point` column; cleared by `EnforceLocationConsent()` when `allow_precise = false`
- **Index:** GIST spatial index (filtered: `WHERE allow_precise = TRUE`)
- **Related:** Allow Precise, Jitter

### Allow Precise

Boolean consent flag controlling whether a Scene or Event stores and exposes its exact location.

- **Default:** `false` — no precise data stored
- **Database:** CHECK constraint ensures `precise_point IS NULL` when `allow_precise = FALSE`
- **Enforcement:** Repository methods call `EnforceLocationConsent()` before every persist
- **Related:** Precise Point, Coarse Geohash

---

## Streaming & Audio

### Stream Room

A LiveKit WebRTC room hosting a live audio session, identified by a unique `room_name` and managed by a `host_did`.

- **Related:** Stream, Participant, LiveKit Token

### Participant

A user connected to a Stream Room with a stable identity derived from their DID.

- **Tracking:** Join/leave events via WebSocket push
- **Related:** Stream Room, DID

### LiveKit Token

A signed JWT credential granting a user permission to connect to a specific Stream Room via WebRTC.

- **Generation:** Server-side, scoped to room + participant identity
- **Expiry:** Short-lived (minutes)
- **Related:** Stream Room, Participant

---

## Identity & AT Protocol

### DID (Decentralized Identifier)

Self-sovereign identity string (e.g., `did:plc:abc123xyz`) used as the primary user identifier. Portable across platforms.

- **Context key:** `internal/auth/context.go` — `SetUserDID()` / `GetUserDID()`
- **JWT claim:** `did` field in access tokens
- **Related:** AT Protocol, Jetstream

### AT Protocol

Decentralized social networking protocol providing user-owned data repositories and portable identity. Subcults ingests records from the AT Protocol firehose.

- **Integration:** Jetstream WebSocket consumer in `internal/indexer/`
- **Data flow:** AT Protocol → Jetstream → Indexer → Postgres
- **Related:** DID, Jetstream, CBOR

### Jetstream

Real-time WebSocket feed of AT Protocol commits. The Indexer connects to this feed and processes incoming records.

- **Client:** `internal/indexer/client.go` — resilient WebSocket with reconnection + backpressure
- **Cursor:** Sequence-based resume for crash recovery
- **Related:** AT Protocol, CBOR

### CBOR (Concise Binary Object Representation)

Binary serialization format (RFC 7049) used by Jetstream for message encoding. Records are decoded from CBOR before database insertion.

- **Parsing:** `internal/indexer/` CBOR decode pipeline
- **Related:** Jetstream, AT Protocol

---

## Abbreviations

| Abbreviation | Expansion                                 | Context                                               |
| ------------ | ----------------------------------------- | ----------------------------------------------------- |
| ADR          | Architecture Decision Record              | `docs/adr/` — rationale for key technical choices     |
| bbox         | Bounding Box                              | Geographic rectangle for spatial queries              |
| CBOR         | Concise Binary Object Representation      | Jetstream message encoding                            |
| CSP          | Content Security Policy                   | HTTP header controlling resource loading              |
| DID          | Decentralized Identifier                  | User identity (AT Protocol)                           |
| FTS          | Full-Text Search                          | PostgreSQL `tsvector`/`tsquery` search                |
| HSTS         | HTTP Strict Transport Security            | Forces HTTPS connections                              |
| PostGIS      | PostgreSQL Geographic Information Systems | Spatial database extension                            |
| R2           | Cloudflare R2                             | S3-compatible media storage                           |
| SLO          | Service Level Objective                   | Performance targets (p95 <300ms API, <2s stream join) |

---

## Entity Relationships

```
Scene ──┬── hosts/contextualizes ── Event ── Stream ── Participant
        ├── Post                         └── Appearance ── Act ── Profile
        ├── Membership ── User (DID)                      └── Tour
        └── Alliance ──→ Scene (directional)

Place ── Venue ── Event
  └── Home Territory ── Act

Scene/Profile ── Audience Relationship ── Participant/Contact
Signal ── Delivery/Engagement/Conversion

Trust Score = f(Alliances, Memberships)
Discovery  = f(Text Relevance, Proximity, Recency, Trust Score)
```

---

## Adding New Terms

When introducing a new domain concept:

1. Add an entry to this glossary with definition, source location, and related terms.
2. On first use in any document, link to this glossary.
3. Prefer established terms over synonyms — consistency reduces confusion.
