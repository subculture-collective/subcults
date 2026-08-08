# Audience, Drops, and Touring Product Plan

**Status:** Proposed canonical product model
**Last updated:** 2026-08-08
**Decision record:** [ADR-007](../adr/0007-scene-signals-touring-relationship-model.md)
**Implementation plan:** [Audience, Drops, and Touring Implementation Plan](../superpowers/plans/2026-08-08-audience-drops-touring.md)

## Purpose

Subcult will combine its privacy-first, location-based scene discovery with an
optional audience activation system for releases, shows, streams, tickets,
merchandise, and other time-bound moments. This expands the original scene and
event plan without turning Subcult into a follower feed or treating every
participant as a marketing contact.

The new model must make touring activity first-class. An artist can have a home
base while appearing at a festival, a tour date, or a one-off show anywhere in
the world. Discovery uses the location of the actual event, while the artist's
home base remains useful cultural context.

## Product Thesis

Subcult connects two loops:

```text
Discovery -> participation -> trust -> collaboration -> renewed discovery
                           |
                           v
             signal -> notification -> action -> conversion
```

The first loop is the core product. The second is an optional organizer tool
that helps scenes and artists activate already-consented relationships. It must
not replace the first loop or silently convert community activity into marketing
permission.

## Product Principles

1. **Occurrence location wins for discovery.** A show is found where it happens,
   not where the artist is based.
2. **Home base is context, not a boundary.** It describes an artist's durable
   affinity to a city or scene but never limits where their activity appears.
3. **Appearances are relationships.** An artist appearing at an event does not
   become a member of, or owned by, the destination scene.
4. **Participation is not consent.** Membership, RSVP, attendance, purchase,
   stream participation, and messaging consent remain separate facts.
5. **Location intent need not reveal device location.** A participant can follow
   a chosen city or region without sharing live or precise coordinates.
6. **Provenance precedes aggregation.** Imported and community-submitted dates
   retain sources, verification state, and conflicts instead of being silently
   merged.
7. **Subcult owns relationship state; providers deliver messages.** Email, push,
   SMS, and social providers are replaceable transports rather than the source
   of truth.

## Canonical Domain Model

### Profile and Act

A public identity for an artist, venue, festival, promoter, collective, label,
or curator. An Act is the creative project that can appear on an Event bill; its
Profile is the public presentation and control surface. A Profile is controlled
by one or more DIDs and can be linked to one or more Scenes without being
identical to a Scene.

Required concepts:

- `kind`: `artist`, `venue`, `festival`, `promoter`, `collective`, `label`, or
  `curator`.
- `home_scene_id`: optional cultural/community affiliation.
- one or more optional, temporal Home Territory declarations.
- verified external identifiers for ticketing, commerce, and music services.
- team and delegation rules for authorized editors.

### Place, Venue, and Home Territory

A Place is a canonical city/market/region and timezone used for discovery. A
Venue is a named hosting location within a Place with independent retention and
public-disclosure rules. An Event points to its occurrence Place and optionally
to a Venue.

A Home Territory is a coarse, declared, and temporal Act-to-Place affinity such
as "Chicago" or "Berlin." It must not store an artist's residence or infer a
private address. It records `valid_from`, optional `valid_to`, visibility, and
the authority that made the declaration. Device location, purchases, RSVPs, IP
addresses, and itinerary gaps must never create or update Home Territory.

### Scene

A place-based or affinity-based underground community. Scenes retain their
sovereignty, membership, visibility, alliances, posts, events, and visual
identity. Scenes may host Events and may be linked to Profiles, but neither
entity subsumes the other.

### Event

A real occurrence at a time and location. Every Event has an occurrence
location independent of the home base of any attached Profile.

Event kinds:

- `show`
- `festival`
- `party`
- `meetup`
- `broadcast`
- `other`

An Event has one or more Hosts. During compatibility migration, the existing
`scene_id` remains the primary host scene; the eventual host relation supports
additional scenes and Profiles.

### Appearance

An Act's participation in an Event. Appearance fields include:

- `act_id`
- `event_id`
- `role`: `headliner`, `support`, `performer`, `dj`, `speaker`, `host`, or
  `other`
- optional `stage_name`, `starts_at`, and `ends_at`
- `status`: `announced`, `confirmed`, `cancelled`, or `completed`
- provenance and verification state

The same relation represents:

- a tour date: Appearance belongs to a Tour;
- a festival appearance: Appearance points to a festival Event and may include
  a stage/set time;
- a one-off away show: Appearance has no Tour membership and occurs outside the
  Profile's home region;
- a home show: Appearance occurs within the Profile's home region.

"Away show" is normally derived by comparing occurrence Place with a visible,
current Home Territory. It is not a separately authored event type.

### Tour

A named grouping of Appearances for one primary Act. A Tour carries
identity, artwork, date range, announcement state, and optional collaborators,
but it does not own location. Each included Event remains independently
discoverable at its occurrence location.

### Festival Program

A festival is an Event whose Appearances form its program. Nested festival days,
stages, and set times are scheduling data, not duplicate Events unless a child
occurrence has meaningfully independent admission, location, or lifecycle.

### Signal

The organizer-facing campaign primitive. "Signal" is the provisional Subcult UI
term; `campaign` may be used internally. A Signal is a time-bound invitation to
take an action and can reference a Profile, Scene, Event, Appearance, Tour,
Post, Stream, or commerce offer.

Examples:

- announce a tour and let people choose cities;
- notify a city when its date goes on sale;
- open a festival presale;
- announce a one-off away show to nearby participants;
- release a recording or start a live stream;
- offer tickets, merchandise, memberships, or community support.

A Signal is immutable after publication except for controlled lifecycle changes
and corrected presentation metadata. Material audience, content, timing, or
offer changes create versioned revisions for auditability.

### Audience Relationship

A relationship between a participant or verified contact and a Scene/Profile.
It records the source and type of connection without collapsing distinct states.

Relationship evidence may include:

- membership;
- RSVP or saved Event;
- attendance confirmation;
- purchase;
- Stream participation;
- explicit interest in a Profile, Tour, city, or kind of Signal;
- explicit messaging consent.

### Contact Point, Consent, and Suppression Ledgers

Email addresses, phone numbers, push subscriptions, and provider-scoped social
identifiers are optional verified Contact Points. They must remain separable
from a DID until a user proves the link.

Consent is an append-only grant/revocation event stream containing:

- scene/profile/program receiving consent;
- channel and purpose;
- grant or revocation action;
- timestamp and region;
- capture source;
- rendered disclosure and policy version;
- actor and evidence reference.

Contact verification and DID-to-contact verification are separate evidence on
the Contact Point and link records. Suppression is a separate append-only
enforcement ledger. Revocation and suppression override all segment membership
and scheduled delivery.

### Consent Scope and Suppression

Consent Scope is a normalized delivery boundary containing:

- sender program: one Scene or Profile;
- channel and purpose;
- optional Tour, Event, Appearance, and/or Place restrictions;
- rendered disclosure/policy version and applicable region.

Scope inheritance is explicit:

- Profile-wide consent can cover that Profile's matching Signals for the same
  channel and purpose unless a narrower revocation or suppression applies.
- Tour consent covers only that Tour; Tour + Place consent covers only dates in
  that occurrence Place.
- Event consent covers only that Event.
- Place consent never implies consent for every sender operating in that Place.
- Consent never crosses to an affiliated or allied Scene/Profile automatically.

At authorization time, the requested Signal scope is matched against every
consent scope that contains it. A scope contains the request only when sender,
channel, and purpose match and each optional restriction on the stored scope
matches the request. Thus Profile-wide consent may apply to a Tour + Place
request, but consent for one Tour never applies to another. The latest state of
each applicable scope is considered; any applicable scope whose latest state is
revoked denies delivery. Delivery is allowed only when at least one applicable
scope remains granted. A new grant must be recorded at the revoked scope before
that boundary can authorize delivery again.

Contact verification proves control of a Contact Point; it does not grant
delivery consent. Where relationship evidence begins on a DID and delivery uses
a Contact Point, a separate auditable and revocable DID-to-contact link must
prove the connection.

Suppression can apply globally to a Contact Point, to a channel, to a sender
program, or to an exact Consent Scope. The most specific grant never overrides
an applicable suppression. Pre-send authorization requires a matching grant,
required Contact Point verification, no later revocation, and no applicable
suppression.

## Touring Discovery Experiences

### Local Map

The map shows Events and Appearances inside the current viewport or selected
area. A visiting artist appears at the event marker for the destination venue or
festival. Cards can show:

- "Visiting from Chicago"
- "Tour date"
- "Festival appearance"
- "One-off show"
- verified source and ticket state

The map must never move the event marker to the artist's home base.

### Artist/Profile Page

Profile pages include:

- declared home scene/Home Territory;
- upcoming appearances grouped into tour, festival, and one-off sections;
- map and chronological list views;
- past appearances where retention and visibility allow;
- controls to follow the Profile, a Tour, or selected cities;
- Signals for releases, tickets, streams, and merchandise.

### Scene Page

Scene pages distinguish:

- locally hosted Events;
- home-scene artists appearing elsewhere;
- visiting artists appearing locally;
- allied-scene recommendations;
- organizer Signals and participant consent controls.

Away appearances should enrich the home Scene without falsely presenting the
destination Event as owned by that Scene.

### Tour Page

A Tour page offers:

- complete verified date list;
- map/list toggle;
- filters for announced, on-sale, sold-out, cancelled, and changed dates;
- city-specific interest and notification controls;
- ticket/source links with provenance;
- additions and material-change history.

### Festival Page

A festival Event offers:

- lineup and set-time views;
- Profile links;
- per-artist and festival-wide interest;
- schedule changes and cancellation states;
- one admission/commerce context unless stages or child Events truly differ.

## Discovery and Ranking

Candidate generation remains based on event occurrence time and location.
Profiles, Tours, and relationship evidence enrich ranking after candidates are
geographically eligible.

Recommended event score:

```text
score = text relevance
      + occurrence proximity
      + time relevance
      + host-scene trust
      + explicit profile/tour/city affinity
      + verified-source confidence
```

Constraints:

- paid promotion never changes trust or provenance scores;
- popularity counts do not dominate local relevance;
- inferred affinity must be explainable and disableable;
- exact user location is not required; selected areas and coarse location are
  sufficient;
- cancelled, duplicate, or disputed dates are excluded or visibly demoted.

## Capture and Activation Journeys

### Tour announcement

1. Artist publishes a Tour and its Appearances.
2. Each Event becomes discoverable at its occurrence location.
3. A Tour Signal invites participants to select cities or dates.
4. Consent is captured for the selected Profile/Tour, channel, and purpose.
5. On-sale and material-change Signals resolve only the eligible audience.
6. Delivery and downstream ticket events append to the engagement ledger.

### Festival appearance

1. Festival or authorized artist adds an Appearance to the festival Event.
2. Source and claimed authority are recorded.
3. Matching or conflicting claims are reviewed rather than silently merged.
4. Local discovery shows the festival; artist discovery shows the Appearance.
5. Participants may follow the artist, the festival, or the individual Event.

### One-off away show

1. Host or artist creates the Event and Appearance without a Tour.
2. The Event is ranked in its occurrence region.
3. The home Scene may surface it as "from this scene, appearing elsewhere."
4. Nearby participants and explicitly interested followers can opt into the
   relevant Signal.

## Provenance, Verification, and Deduplication

Every imported or submitted Event, Appearance, Tour, and ticket link stores:

- source system and stable external identifier;
- source URL where safe to retain;
- first-seen and last-seen timestamps;
- submitting DID or integration;
- verification state: `unverified`, `claimed`, `verified`, `disputed`, or
  `rejected`;
- raw source hash and normalized fields;
- supersession/correction relation.

Candidate duplicates can be generated from normalized venue, start time,
Profiles, and source identifiers. Automatic merging is allowed only for exact,
stable source identities. Ambiguous matches remain separate for review. A
verified correction never destroys the original claim or provenance.

## Privacy and Safety

- Keep Profile home region at city/region granularity; never infer a residence.
- Venue/event location follows the existing coarse-by-default model. Public
  ticketed venues may opt into precise display; underground locations may
  remain coarse or reveal details only to authorized participants.
- Saved cities are interest preferences, not evidence of residence or presence.
- Do not expose attendee lists by default.
- Do not share audience segments across allied Scenes without explicit scope.
- Do not use purchases, location, or trust to infer sensitive traits.
- Contact export preserves consent scope and suppression state.
- Automated sends, discounts, audience selection, and rights requests require
  human-configurable limits and audit logs.

## Product Boundaries

### Included

- local and travel-aware event discovery;
- artist/venue/festival/promoter Profiles;
- Tours and Appearances;
- Signals, consented audiences, segmentation, and delivery orchestration;
- native RSVP, push/email activation, and Stripe attribution;
- provenance-preserving imports and provider adapters.

### Deferred until the core is proven

- in-house SMS carrier operations;
- Instagram or WhatsApp automation;
- network-wide anti-broker scoring;
- autonomous discounts or message sending;
- cross-customer enrichment from opaque third-party data;
- inferred home bases or unverified identity linkage.

## Delivery Phases

| Phase | Outcome | Evidence required |
| --- | --- | --- |
| 0. Durable foundation | Authenticated Postgres-backed API and current source/runtime truth | Persistence and restart tests; auth smoke; migrations |
| 1. Profiles and appearances | Profiles, Events, Appearances, Tours, festival programs, provenance | API/schema tests; map/list discovery of away dates |
| 2. Signals and consent | Public Signal pages, verified Contact Points, consent ledger, web push | Revocation/suppression tests; end-to-end opt-in and delivery |
| 3. Audience workspace | Explainable segments from native relationship evidence; email adapter | Segment fixtures; scheduling/delivery/engagement tests |
| 4. Commerce and imports | Stripe attribution plus selected ticketing/commerce import adapters | Signed webhook, idempotency, reconciliation, provenance tests |
| 5. Advanced channels | SMS/social adapters where operating capacity exists | Registration, compliance, deliverability, opt-out propagation |
| 6. Assisted automation | Drafting and recommendations with human approval and auditability | Evaluation set, safety limits, approval and rollback evidence |

## Success Measures

Product success is not raw contact-list growth. Track:

- percentage of published Events with verified occurrence and Appearance data;
- discovery-to-detail and detail-to-RSVP rates by local/away context;
- percentage of participants using manual city interest rather than device
  location;
- consent verification and revocation propagation latency;
- repeat participation across Scenes and locations;
- attributable ticket/support conversion with the model and window disclosed;
- correction and duplicate-resolution accuracy;
- organizer time to publish a tour or one-off date;
- participant complaint, spam, and notification-disable rates.

## Open Product Decisions

These decisions are intentionally deferred to implementation discovery rather
than silently assumed:

1. Final participant-facing name for Signal.
2. Which Profile kinds ship in the first slice beyond artists and venues.
3. First external ticketing integration, chosen from actual beachhead users.
4. Whether a festival's multi-day schedule uses child Events or structured
   program days in the first implementation.
5. Threshold and regional rules for displaying the derived "away" label.
6. Whether allied Scenes can propose, rather than publish, Appearances for each
   other.
