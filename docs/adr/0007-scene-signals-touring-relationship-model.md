# ADR-007: Separate Profiles, Event Occurrences, Appearances, and Signals

**Status:** Accepted
**Date:** 2026-08-08

## Context

The original Subcult model makes Scene the primary discovery entity and requires
each Event to belong to a Scene. That supports local scene discovery but cannot
accurately express an artist with a home base who plays a tour date, festival,
or one-off show elsewhere. Treating the artist as the destination Scene would
misstate community ownership and place the Event at the wrong cultural locus.

Adding Drop-style audience activation also creates a risk that memberships,
RSVPs, purchases, and location activity become an undifferentiated marketing
profile. That conflicts with scene sovereignty and privacy-first consent.

## Decision

Subcult will use separate domain concepts:

1. **Profile** identifies a public artist, venue, festival, promoter,
   collective, label, or curator identity. **Act** is the creative project that
   can appear on an Event bill.
2. **Place** identifies a canonical city/market/region; **Venue** identifies a
   hosting location with separate retention and disclosure rules; **Home
   Territory** is a coarse, declared, temporal Act-to-Place affinity.
3. **Event** represents an occurrence with its own time, Place, optional Venue,
   hosts, and lifecycle.
4. **Appearance** relates an Act to an Event, including performance role,
   set time, state, and provenance.
5. **Tour** groups Appearances but never owns or overrides Event location.
6. **Signal** is a time-bound, versioned invitation to act on a Profile, Scene,
   Event, Appearance, Tour, Stream, Post, or offer.
7. **Audience Relationship** records distinct participation evidence, while a
   **Consent Scope** and its grant/revocation events separately authorize a
   channel, purpose, sender, and optional touring/place boundary. Contact
   verification is evidence, and suppression is a separate enforcement ledger.

The existing `events.scene_id` remains required during migration as the primary
host Scene. New host relations can later support multiple Scenes and Profiles.
Away-from-home status is derived from Event occurrence Place versus an Act's
visible current Home Territory; it is not a separately authored event kind.

## Consequences

### Positive

- Tour dates and festival appearances are discoverable where they occur.
- Artist home-scene context survives without asserting destination ownership.
- Multi-act Events do not need duplicate records for each artist.
- One model covers tours, festivals, one-offs, home shows, and visiting artists.
- Audience activation can reuse community evidence without treating it as
  messaging consent.
- Delivery providers remain replaceable because consent and campaign state stay
  inside Subcult.

### Negative

- More relations and moderation states increase schema and UI complexity.
- Existing Event APIs must migrate from a single implicit owner toward explicit
  host and appearance relations.
- Imports need provenance, deduplication, and conflict-resolution workflows.
- Profile control and delegation introduce an additional authorization surface.

### Neutral

- Scene remains the primary community/trust unit.
- Existing coarse/precise Event location rules continue to apply.
- Signal is a provisional UI term and can change without changing the internal
  campaign boundary.

## Alternatives Considered

### Make every artist a Scene

Rejected because an artist identity is not necessarily a community, venue, or
trust domain. It also fails for multi-act bills and visiting artists.

### Copy each touring Event into the artist's home Scene

Rejected because duplicates split RSVPs, payments, corrections, and provenance,
and may show the wrong organizer or location.

### Store `is_away_show` on Event

Rejected because "away" is relative to a Profile, not intrinsic to an Event. A
festival can be local for one artist and away for another.

### Treat every RSVP or purchase as marketing consent

Rejected because transactional or community participation does not establish
permission for every sender, channel, or purpose.
