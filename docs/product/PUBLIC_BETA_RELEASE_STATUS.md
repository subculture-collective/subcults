# Public Beta Release Status

Status date: 2026-08-08

This is the executable release contract for the web public beta. It replaces
older claims that the placeholder frontend or in-memory API was production
ready. A checked source capability is not the same as a configured staging or
production capability.

## Release shape

The beta is a location-first music discovery and creator operations product:

- participants browse scenes, local events, visiting artists, tour stops,
  festivals, and one-off appearances without granting device location;
- public maps consume only the server-approved occurrence projection;
- authenticated participants use passwordless email and opaque rotating
  refresh sessions;
- approved creators author Scenes, Profiles/Acts, Events, Tours, Appearances,
  and Signal drafts in Studio;
- precise protected venue details require a separate explicit access grant;
- browser permission, RSVP, membership, purchases, and Signal delivery consent
  remain distinct records.

Streaming, payments, advanced ranking controls, and broad CRM automation are
not release navigation. SMS, Instagram, WhatsApp, and autonomous sending remain
deferred.

## Implemented and locally verified

| Capability | Source evidence | Local evidence |
| --- | --- | --- |
| Passwordless identity | Migration 38; `internal/identity`; v1 auth routes | Focused identity, middleware, API, and command tests pass |
| Creator approval | Role/request history plus admin review API and UI | Creator-focused Go tests pass |
| Public frontend overhaul | New editorial system, IA, discovery/detail/auth/participant/admin pages | ESLint and Vite production build pass |
| Creator Studio | Creator-gated forms and APIs for scenes, profiles, events, tours, appearances, Signals | Touring mutation flow and focused API tests pass |
| Touring discovery | Event occurrence + Act + Appearance + Tour projections | Touring domain/API tests pass |
| Protected locations | Coarse public projection plus explicit precise-location grants | Deny-before-grant and grant/revoke tests pass |
| Web push | Encrypted subscriptions, authenticated revoke, VAPID sender, privacy-safe service worker | Migration 39 rollback/reapply, Go tests, and 47 focused web notification tests pass |
| Database evolution | Additive migrations 0 through 39 | Fresh PostGIS up, down-one, up-one reaches version 39 |
| Backend regression gate | Whole repository | `go test ./... -count=1` passes in the libvips-enabled build image |
| Release automation | Required CI, security scans, reversible migration check, fail-closed deploy migrations | Compose renders, workflow YAML parses, deploy script passes `bash -n` |

Dependency remediation removed all high and critical npm audit findings. Two
moderate React Router advisories remain because the available fix is a breaking
major-version upgrade and must be handled as an explicit compatibility slice.

## Blocking release gates

### 1. Durable core repositories

The production API intentionally exits with
`ErrDurableRepositoriesUnavailable`. Identity, protected-location grants, and
web-push subscriptions have SQL adapters, but Scene, Event, RSVP, Touring,
Audience, and Signal runtime repositories are still in-memory. Shipping around
this guard would create silent data loss and is prohibited.

Required evidence:

1. SQL adapters implement the existing repository contracts.
2. Restart tests prove authored records, consent, and revisions survive.
3. Concurrent edit/version tests prove stale Studio writes receive a conflict.
4. Readiness checks the same database used by the live handlers.

### 2. Frontend suite reconciliation

The new app lints and builds, but the full legacy Vitest run currently reports
24 failing files / 121 failing assertions. Most assertions target retired
password login, placeholder navigation, streaming release routes, or render
query-based pages without their provider. Several unrelated older failures also
remain in theme, i18n, session-replay, stream accessibility, settings, and map
fixtures. The deploy gate now correctly fails on this state.

Required evidence:

1. Replace retired placeholder contracts with public-beta route and UX tests.
2. Preserve focused privacy, consent, auth, protected-location, and Studio tests.
3. Reach a green full Vitest run; do not lower coverage to conceal deletions.

### 3. Configured staging and provider evidence

No provider credential or deployment was used during this implementation.
Staging needs a real database, `PUBLIC_WEB_URL`, contact encryption/HMAC keys,
Postmark transactional credentials, matching server/client VAPID keys, a VAPID
subject, allowed origin, and MapTiler key. Secrets must not enter Git.

Required evidence:

1. Magic link received and consumed once; replay and expiry rejected.
2. Refresh rotation and logout survive page reload and API restart.
3. Web push opt-in, delivery, click-through, revoke, and post-revoke suppression.
4. Protected venue stays coarse publicly and reveals only to an authorized user.
5. A two-city tour, festival appearance, and one-off appear correctly in list
   and map views after restart.
6. Browser accessibility and responsive checks on the deployed artifact.

## Release decision

Current verdict: **not releasable**. The source implementation has advanced to
an integrated beta shape, but bypassing the durable repository guard or the
failing deploy gate is not an acceptable release path.

The next implementation slice is durable Scene/Event/RSVP and Touring query
repositories, followed by Audience/Signal persistence, test-suite
reconciliation, and configured staging qualification.
