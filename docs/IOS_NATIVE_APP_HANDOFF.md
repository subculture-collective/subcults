# Subcult Native iOS App Handoff

This document is the product and technical handoff for an iOS developer building a native Subcult app. Assume the backend will expose all required REST endpoints, LiveKit token issuance, upload signing, payment handoff, and notification registration needed by the app.

The product model for Profiles/Acts, Places/Venues, Appearances, Tours,
festivals, one-off shows, Signals, and consent is defined in
`docs/product/AUDIENCE_DROPS_AND_TOURING.md`. Endpoint names for those planned
domains remain provisional until they are added to `docs/openapi.yaml`.

## 1. Product Summary

Subcult connects underground and local music communities through privacy-first discovery of scenes, events, posts, alliances, and live audio. The app should feel like a trust-based map and community utility, not a follower feed or popularity network.

Core principles:

- **Presence over popularity:** prioritize nearby, current, and trusted activity.
- **Scene sovereignty:** each scene controls its identity, membership, visibility, and settings.
- **Human discovery:** ranking should blend text relevance, proximity, recency, and trust.
- **Privacy first:** never expose precise scene/event locations unless explicit consent exists.
- **Decentralized identity:** user identity is DID-based and aligned with AT Protocol ingestion.

Primary entities:

- **Scene:** underground music community, venue, collective, or curator space.
- **Event:** time-specific gathering belonging to a scene.
- **Post:** text/media update attached to a scene or event.
- **Stream:** LiveKit live audio room associated with a scene/event.
- **Membership:** user participation in a scene with role/status.
- **Alliance:** directional trust relationship between scenes.
- **Trust score:** 0.0-1.0 credibility signal used in ranking.
- **Profile/Act:** DID-controlled public identity and creative project that can appear at Events.
- **Place/Venue:** occurrence geography and optional named host location, independent of an Act's Home Territory.
- **Appearance:** an Act's participation in an Event; may be a tour stop, festival set, home show, or one-off.
- **Tour:** named grouping of Appearances; never the source of occurrence location.
- **Signal:** scoped, time-bound invitation for a release, show, on-sale, stream, or offer.
- **Audience relationship/consent:** distinct evidence and permission records; RSVP, membership, and purchase do not imply delivery consent.

Reference docs:

- `README.md`
- `docs/ARCHITECTURE.md`
- `docs/openapi.yaml`
- `docs/GLOSSARY.md`
- `docs/PRIVACY.md`
- `docs/api/SEARCH_ENDPOINT.md`
- `docs/api/SCENE_VISIBILITY.md`
- `docs/streaming-mini-player.md`

## 2. Recommended iOS Stack

- **Language/UI:** Swift 6+, SwiftUI.
- **Concurrency:** async/await with structured cancellation.
- **Navigation:** `NavigationStack` plus tab-based root navigation.
- **Networking:** `URLSession` with a typed API client and shared auth interceptor.
- **Persistence:** SwiftData or SQLite for lightweight cache; Keychain for sensitive tokens.
- **Maps:** MapLibre Native/MapLibre Maps SDK for iOS to match MapTiler/MapLibre web behavior. Use MapKit only if custom tile styling is explicitly deprioritized.
- **Location:** Core Location, requested only for user-driven discovery.
- **Audio streaming:** LiveKit iOS SDK.
- **Push notifications:** APNs. Do not copy web push implementation directly.
- **Payments:** Stripe iOS SDK where native checkout is available; otherwise use a secure browser handoff for Stripe Connect onboarding.
- **Media:** PhotosUI, AVFoundation where needed, background-safe upload tasks if large media is supported.
- **Observability:** client telemetry/error endpoint with local redaction before upload.

## 3. MVP App Scope

### Must Have

1. Authentication and session restore.
2. Map-first home/discovery experience for scenes and events.
3. Global search across scenes, events, and posts.
4. Scene detail page with feed, events, membership action, visibility-aware states.
5. Event detail page with RSVP and stream entry points.
6. Live audio stream room with persistent mini-player behavior.
7. Settings/account page with notification preferences, privacy controls, export/delete entry points.
8. Scene owner tools for basic scene settings, palette/identity, membership approval, event creation/editing.
9. Native push notification opt-in and APNs token registration.
10. Robust offline/loading/error states.
11. Touring discovery for visiting Acts, festival appearances, tour dates, and one-off away shows.
12. Explicit Signal interest and channel/purpose consent controls.

### Should Have

1. Scene creation flow.
2. Post creation with media attachment upload.
3. Alliance management for scene owners.
4. Stripe payment onboarding/checkout handoff.
5. In-app notification center.
6. Cached recently viewed scenes/events/search results.

### Out of Scope for First Native Release

1. Admin dashboard.
2. Full AT Protocol authoring UX beyond what backend exposes.
3. Advanced moderation tooling, unless required before public launch.
4. ML/personalized ranking controls.

## 4. Information Architecture

Recommended root tabs:

1. **Map**
   - Nearby scenes/events.
   - Search entry.
   - Map/list toggle.
   - Location permission education.
2. **Search**
   - Global search.
   - Filters: type, genre/tags, date, distance, Act, Tour, festival, and local/visiting status.
3. **Streams**
   - Live/featured audio rooms.
   - Current stream state.
4. **My Scenes**
   - Owned scenes.
   - Memberships.
   - Pending requests for owners.
5. **Settings**
   - Profile/account.
   - Notifications.
   - Privacy/diagnostics.
   - Export/delete account.

Deep links should support:

- `subcult://scenes/{id}`
- `subcult://events/{id}`
- `subcult://streams/{id}`
- `subcult://profiles/{id}`
- `subcult://tours/{id}`
- `subcult://signals/{id}`
- `subcult://search?q=...`

## 5. Core Screens and Behaviors

### 5.1 Auth

Screens:

- Login/sign-in.
- Session restoring splash state.
- Logged-out explanation for protected actions.

Behavior:

- Store refresh credentials in Keychain.
- Keep access token in memory when possible.
- Refresh access token automatically on 401, then retry the original request once.
- If refresh fails, clear session and show login.
- Never log tokens, cookies, DID values, or auth headers.

Expected endpoints:

- `POST /api/auth/login`
- `POST /api/auth/refresh`
- `POST /api/auth/logout`
- `GET /api/auth/me`

User model should at minimum include:

- `did: String`
- `role: user | admin`
- display/profile fields if backend provides them.

### 5.2 Map Home

Purpose: default discovery surface for scenes/events near a location or map viewport.

Requirements:

- Show only the server-provided public occurrence projection. It may be
  coarse/jittered or approved precise; never select from raw stored coordinates
  on the client.
- Support clustering or density-aware marker display.
- Support map pan/zoom with debounced bbox search.
- Provide list sheet for visible results.
- Allow user location permission but do not require it.
- Provide manual city/area search fallback.
- Use stable marker positions from backend jittered data; do not attempt to infer exact venues.
- Place touring markers at the Event occurrence location, never at an Act's Home Territory.
- Distinguish locally hosted Events, visiting Acts, tour dates, festival appearances, and one-offs in cards/list filters.
- Support an explicit city/area interest without requiring device location.

Expected endpoints:

- `GET /search/scenes?q=&bbox=&lat=&lon=&genres=&limit=&cursor=`
- `GET /search/events?q=&bbox=&lat=&lon=&genres=&starts_after=&starts_before=&limit=&cursor=`
- `GET /search/global?q=&bbox=&limit=&cursor=`

Result cards should show:

- Name/title.
- Type: scene or event.
- Tags/genres.
- Approximate area, never exact address unless `allow_precise` permits it.
- Trust score or trust badge where available.
- Upcoming time for events.
- Host Scene/Venue separately from performing Acts.
- Tour/festival/one-off context and verification/source state when available.

### 5.3 Search

Requirements:

- Debounced text input.
- Results grouped by scenes, events, posts.
- Recent searches stored locally only.
- Filters for tags/genres, distance, date, type.
- Empty states that explain how discovery works.
- Filters for Act, Tour, festival, locality (`local | visiting | any`), and chosen Place.

Expected endpoints:

- `GET /search/scenes`
- `GET /search/events`
- `GET /search/posts`
- `GET /search/global`

Ranking should be treated as backend-owned. The client should not re-sort in ways that undermine trust/proximity/recency ranking, except for local grouping or user-selected sort options explicitly provided by the API.

### 5.4 Scene Detail

Requirements:

- Header: name, description, visual identity/palette, tags, approximate location.
- Visibility badge: public, members-only, hidden/owner-only when visible to owner.
- Membership action:
  - Request membership.
  - Show pending/active/rejected status.
  - Owner sees membership management.
- Feed of posts.
- Events attached to the scene.
- Active or upcoming live stream entry point.
- Alliance/trust summary if available.

Expected endpoints:

- `GET /scenes/{id}`
- `GET /scenes/{id}/feed`
- `POST /scenes/{id}/membership/request`
- `GET /trust/{sceneId}`
- `GET /events?scene_id={id}` or equivalent.

Visibility handling:

- Treat `404` for a protected scene exactly like not found. Do not reveal that a private/hidden scene exists.
- Public scenes are visible to all.
- Members-only scenes are visible only to owners and active members.
- Hidden scenes are visible only to owner.

### 5.5 Event Detail

Requirements:

- Title, scene, starts/ends time, tags, flyer/media, description.
- Location display follows same precise/coarse privacy rules.
- RSVP control.
- Capacity/ticket/payment information if provided.
- Stream entry point if live or scheduled.
- Host Scene/Venue and occurrence Place, distinct from each Act's Home Territory.
- Lineup/Appearances with billing role, set time, Tour link, and cancellation/change state.
- Signal opt-in for this Event, related Tour, performing Act, or selected city where supported.

Expected endpoints:

- `GET /events/{id}`
- `POST /events/{id}/rsvp`
- `GET /events/{id}/feed`
- `POST /payments/checkout` if paid ticketing is enabled.
- `GET /events/{id}/appearances` or equivalent planned contract.

### 5.5A Profiles, Tours, and Festivals

Requirements:

- Profile detail with declared Home Territory and affiliated Scenes.
- Chronological and map views of upcoming Appearances.
- Tour detail with per-date state and city-specific interest controls.
- Festival program grouped by day/stage without duplicating the festival Event.
- Clear source, verification, correction, postponed, and cancelled states.
- Never infer Home Territory from device location, itinerary, purchase, or RSVP.

Planned endpoint families:

- `GET /profiles/{id}` and `GET /profiles/{id}/appearances`
- `GET /tours/{id}` and `GET /tours/{id}/appearances`
- `GET /events/{id}/appearances`
- `GET /places/{id}` and `GET /venues/{id}` where public

### 5.6 Live Audio Streams

Use LiveKit iOS SDK. The native app should mirror the web mini-player concept: a LiveKit room connection survives navigation and collapses into a persistent player.

Requirements:

- Join stream from scene/event/detail cards.
- Fetch short-lived LiveKit token from backend.
- Connect to room.
- Show participants and roles.
- Mute/unmute local microphone.
- Speaker/listener role UI if backend supports it.
- Show connection quality.
- Auto-reconnect with bounded exponential backoff.
- Leave room explicitly and clean up all audio resources.
- Persistent mini-player at bottom of app while connected.

Expected endpoints:

- `GET /streams/{id}`
- `POST /streams/{id}/join`
- `POST /streams/{id}/leave`
- `POST /streams/{id}/end` for hosts/organizers.
- `GET /streams/{id}/participants`
- `POST /livekit/token`

Organizer controls where authorized:

- mute participant: `POST /streams/{id}/participants/{participantId}/mute`
- kick participant: `POST /streams/{id}/participants/{participantId}/kick`
- feature participant: `POST /streams/{id}/featured_participant`
- lock room: `POST /streams/{id}/lock`

### 5.7 Scene Owner Tools

Requirements:

- List owned scenes.
- Edit scene name, description, tags, visibility, coarse/precise location consent.
- Edit scene palette/visual identity.
- Create/edit/cancel events.
- Review pending membership requests.
- Approve/reject members.
- Start/end streams if supported.
- Payment onboarding handoff if owner enables payments.

Expected endpoints:

- `GET /scenes/owned`
- `POST /scenes`
- `PATCH /scenes/{id}`
- `PATCH /scenes/{id}/palette`
- `DELETE /scenes/{id}`
- `POST /events`
- `PATCH /events/{id}`
- `POST /events/{id}/cancel`
- `POST /scenes/{id}/membership/{userId}/approve`
- `POST /scenes/{id}/membership/{userId}/reject`
- `POST /payments/onboard`

### 5.8 Posts and Media Uploads

Requirements:

- Compose text posts for a scene or event if user is authorized.
- Attach image/audio/media where supported.
- Use backend-signed upload URLs.
- Assume backend strips EXIF/location metadata, but the app should still avoid displaying or relying on local EXIF.
- Show upload progress, retry, and cancel.

Expected endpoints:

- `POST /uploads/sign`
- `POST /posts`
- `PATCH /posts/{id}`
- `DELETE /posts/{id}`

### 5.9 Notifications

Native iOS should use APNs, not browser Web Push.

Requirements:

- Ask for notification permission only from Settings or a contextual opt-in screen. Never prompt automatically on first launch.
- Register APNs device token with backend.
- Provide clear preference categories:
  - new events from joined scenes,
  - stream started,
  - membership approved/rejected,
  - scene owner/admin alerts.
  - followed Act/Tour dates in selected cities,
  - festival lineup/set-time changes,
  - Signal on-sale/release reminders explicitly chosen by the user.
- Treat operating-system permission, verified Contact Point, and sender/channel/purpose consent as separate states.
- Never subscribe a user to marketing merely because they joined a Scene, RSVPed, attended, streamed, or purchased.
- Allow opt-out and token deletion.
- Deep link notification taps to scene/event/stream.

Expected endpoints:

- `POST /api/notifications/register-apns` or equivalent.
- `DELETE /api/notifications/register-apns` or equivalent.
- `GET/PATCH /api/notifications/preferences` or equivalent.

## 6. API Client Requirements

Base URL should be environment-configurable.

Implement one shared API layer with:

- Typed request/response DTOs matching `docs/openapi.yaml`.
- `Authorization: Bearer <accessToken>` injection.
- 10-15 second default timeout.
- Automatic access token refresh on 401.
- Deduped refresh if multiple requests fail concurrently.
- Idempotent retry for GET/PUT/DELETE/HEAD/OPTIONS on network/5xx/timeout errors.
- No automatic retry for POST/PATCH unless endpoint explicitly supports idempotency keys.
- Normalized error model.
- Rate-limit handling for `429` and `Retry-After`.

Recommended normalized error:

```swift
struct APIError: Error {
    let status: Int?
    let code: String
    let message: String
    let retryAfter: TimeInterval?
    let requestID: String?
}
```

Expected backend error shape:

```json
{
  "error": {
    "code": "validation_error",
    "message": "bbox parameter is required"
  }
}
```

## 7. Data and DTO Notes

Use backend JSON field names directly or map them consistently.

Important common fields:

- IDs are UUID strings unless noted.
- User identity is DID string, e.g. `did:plc:abc123`.
- Dates are ISO-8601 UTC strings.
- Coordinates use WGS84 latitude/longitude.
- Search pagination may use `cursor` and `next_cursor`.

Scene fields likely include:

- `id`
- `did`
- `name`
- `description`
- `owner_did`
- `visibility`: `public | private | unlisted`
- `allow_precise`
- `precise_point` when permitted
- `jittered_centroid` for public map display/search
- `coarse_geohash`
- `tags`
- `palette`
- `created_at`, `updated_at`

Membership fields likely include:

- `scene_id`
- `user_did`
- `role`: `owner | curator | member | guest`
- `status`: `pending | active | rejected`
- `trust_weight`

Stream fields likely include:

- `id`
- `scene_id`
- `room_name`
- `host_did`
- `status`: `pending | live | ended`
- `started_at`, `ended_at`

Planned touring DTOs should preserve:

- `Profile`/`Act`: identity, kind, public Home Territory, affiliated Scenes.
- `Place`/`Venue`: timezone, coarse/public projection, disclosure state.
- `Appearance`: `event_id`, `act_id`, optional `tour_id`, billing role, set time, status, provenance.
- `Tour`: primary Act, title, lifecycle, date range, artwork, verification state.
- `Signal`: target references, revision, lifecycle, audience summary, explicit consent call to action.

## 8. Privacy and Security Requirements

Non-negotiable requirements:

- Never display exact location unless backend indicates precise location is allowed.
- Prefer `jittered_centroid` and `coarse_geohash` for maps/search.
- Do not ask for precise user location until the user invokes location-based discovery.
- Provide manual search if location permission is denied.
- Treat `404` on private/hidden content as a generic not-found state.
- Store refresh/session secrets only in Keychain.
- Redact tokens, DIDs, emails, and auth headers from logs/telemetry.
- Do not log request bodies for auth, uploads, payments, or account deletion.
- Use ATS/HTTPS only for production.
- For delete/export flows, require re-authentication if backend supports it.

Telemetry:

- Send client errors to `POST /api/log/client-error` or equivalent.
- Send performance/usage telemetry only according to user settings.
- Always redact PII before sending diagnostics.

## 9. Offline, Cache, and State

Recommended local cache:

- Current authenticated user.
- Recently viewed scenes/events.
- Recent search queries.
- Last map viewport and visible results.
- Current stream metadata, not credentials.

Cache invalidation:

- Prefer short TTLs for discovery/search.
- Refresh detail screens on foreground.
- Do not cache private scene content after logout.
- Clear all user-scoped cache on logout/account deletion.

Offline behavior:

- Show cached read-only scene/event details when available.
- Disable protected mutations with clear offline state.
- Queueing writes is not required for MVP unless explicitly requested.

## 10. Accessibility and UX Requirements

- Support Dynamic Type.
- Support VoiceOver labels for map markers, stream controls, and search results.
- Provide non-map list alternative for discovery.
- Use sufficient contrast in custom scene palettes.
- Avoid color-only status indicators.
- Provide haptic feedback sparingly for major actions: RSVP, join stream, membership request.
- Show clear loading, empty, unauthorized, rate-limited, and offline states.
- All destructive actions need confirmation.

## 11. Suggested Milestones

### Milestone 1: App Foundation

- Project setup.
- Environment config.
- Typed API client.
- Auth/session restore.
- Root tabs/navigation.
- Error/loading components.

### Milestone 2: Discovery

- Map home.
- Search endpoints.
- Scene/event cards.
- Scene detail and event detail read-only flows.

### Milestone 3: Community Actions

- Membership request/status.
- RSVP.
- Feed/post display.
- Basic owned scene list.

### Milestone 3A: Audience and Touring

- Profile/Act, Place/Venue, Appearance, Tour, and festival DTOs.
- Visiting/local/tour/festival discovery filters.
- Profile, Tour, and festival program screens.
- Signal interest, consent, and preference controls.
- Source/verification/change states.

### Milestone 4: Streaming

- LiveKit token fetch.
- Join/leave room.
- Participant list.
- Persistent mini-player.
- Reconnect and audio lifecycle handling.

### Milestone 5: Owner Tools and Native Integrations

- Scene/event edit flows.
- Membership approve/reject.
- APNs registration/preferences.
- Media upload.
- Payment onboarding/checkout handoff.

### Milestone 6: Hardening

- Privacy review.
- Accessibility pass.
- Offline/cache polish.
- Telemetry/error logging.
- App Store readiness.

## 12. Testing Expectations

Unit tests:

- API client refresh/retry/rate-limit behavior.
- DTO decoding for all major endpoints.
- View models for search, auth, scene detail, stream state.
- Privacy mapping rules.

Integration tests:

- Login/session restore/logout.
- Search and map result load.
- Membership request.
- RSVP.
- LiveKit join/leave using staging room.
- APNs token registration in a staging environment.

Manual QA checklist:

- Location permission denied, approximate, and precise states.
- Private/unlisted scene access returns generic not-found.
- Stream survives navigation and ends cleanly.
- AirPods/Bluetooth route changes.
- Background/foreground during active stream.
- Network loss during search and streaming.
- Token expiry during active use.
- Logout clears protected cache.

## 13. Open Decisions for Product/Backend

Resolve before implementation starts or document assumptions in the iOS repo:

1. Native auth flow details: password, magic link, OAuth/AT Protocol, or all of the above.
2. APNs endpoint names and notification preference schema.
3. Exact scene/event/post DTOs for native, generated from OpenAPI or hand-maintained.
4. Whether map uses MapLibre custom tiles or MapKit.
5. Whether payments use native Stripe PaymentSheet or browser checkout only.
6. Which owner/moderator controls are included in the first App Store release.
7. App deep link/universal link domain.
8. Minimum supported iOS version.
9. First shipped Profile kinds and first ticketing/import provider.
10. Final participant-facing name for Signal.
11. Multi-day festival representation and derived away-label threshold.

## 14. Acceptance Criteria for First Native Build

The first native iOS release is acceptable when:

- A user can log in, close/reopen the app, and remain authenticated.
- A logged-out user can browse public map/search results.
- A logged-in user can search, view scene/event details, request membership, and RSVP.
- The map never exposes non-consented exact locations.
- A user can join a live stream, navigate away, control audio from a mini-player, and leave cleanly.
- A scene owner can perform the agreed owner MVP actions.
- Push notification permission is explicit and reversible.
- All major errors have user-readable states.
- Tokens and private data are stored/redacted according to this document.
- Core flows pass simulator and physical-device QA.
