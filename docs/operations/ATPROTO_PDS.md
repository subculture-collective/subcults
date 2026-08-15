# AT Protocol and PDS Operations

## Runtime contract

Public Studio records are canonical on the linked creator PDS. PostgreSQL keeps
private drafts and a validated discovery projection. A successful PDS write
returns `202 Accepted` and is not rolled back when projection is delayed.
Tap is the durable tracker/backfill path; Jetstream remains a shadow comparator
and disabled fallback after promotion.

OAuth is a confidential Go backend-for-frontend flow using Indigo, PKCE, DPoP,
PAR, encrypted server-side request/session storage, and P-256 client signing.
Base linking requests `atproto`; Studio upgrade requests the explicit
`repo:tv.subcult.*` collections. Refresh and publication mutations use Redis
locks. Arbitrary valid PDS hosting is supported; URL discovery rejects HTTP,
credentials, redirects, loopback, private, link-local, and rebinding-to-private
destinations.

## Local secrets

Generate independent values without printing them:

```bash
go run ./cmd/atproto-keygen
```

The command preserves existing non-empty values and writes `deploy/.env` mode
`0600`. Do not reuse JWT, contact encryption/HMAC, or VAPID secrets. Required
AT Protocol values are the P-256 private key and public JWKS metadata, 32-byte
session encryption key, private provisioner token, and Tap admin password.

## Feature switches

- `ATPROTO_OAUTH_ENABLED` enables linking, publication, projection status, Tap
  intake, and reconciliation. It requires PostgreSQL and Redis.
- `PDS_SIGNUP_ENABLED` independently enables guarded account provisioning and
  defaults to false.
- `PDS_SIGNUP_DAILY_CAP` defaults to 25.
- Removing the Tap Compose profile stops tracking without disabling public
  reads or local drafts. Jetstream remains independently configurable.

Run `pds-provisioner` only on the private Compose network. It holds the PDS
administrator password and exposes invite issuance plus health; it has no
public port or general administrator proxy. The browser submits its selected
handle, invite, email, and password directly to the PDS. The Subcults API must
never receive or log the PDS password.

Bluesky PDS invitation codes are single-use but do not expose a native
server-enforced expiry. Subcults records and displays a ten-minute invitation
window, but that alone cannot invalidate an unused code at the PDS. Public
provisioning must remain disabled until the dedicated PDS boundary adds an
expiry-capable invite broker or the upstream PDS supports revocation/expiry.

## Publication and synchronization

1. Save a private draft with its optimistic `version`.
2. Link an AT Protocol account and upgrade the exact repository scopes.
3. Publish through `POST /api/v1/studio/atproto/publish`.
4. Persist the returned AT URI/CID as `awaiting_projection`.
5. Tap posts a normalized event to the private authenticated sync endpoint.
6. Validate the record and matching CID, persist a digest-only observation,
   then mark the public projection complete.
7. After 15 seconds, reconciliation fetches `getRecord` directly from the
   authoritative public PDS through the SSRF-safe transport. Invalid, unmapped,
   or conflicting records go to quarantine.

Tap is built from reviewed Indigo commit
`8b43a326dbbb394f63b6d68761553cdfe25532de`. Upgrades require lexicon,
OAuth, and parity tests. OAuth's Go dependency is separately pinned in
`go.mod` for the repository's Go 1.26.6 toolchain.

## Release gates and monitoring

Internal NUC accounts may use the current PDS. Public signup stays disabled
until a dedicated SSD-backed workload has reservations, monitored backups and
restore evidence, plus the specified 250-repository/25-concurrent-user/10,000-
record qualification. Require p95 PDS writes under 1 second, p95 projection
under 5 seconds, and p99 projection under 15 seconds.

Monitor PDS health/storage/write latency; OAuth and refresh failures; active
links; invite issuance/rejection; Tap checkpoint age; projection latency/parity
gaps/quarantine; reconciliation/CID conflicts; and backup/restore-test age.
Promotion requires seven consecutive days of unexplained-gap-free Tap and
Jetstream parity. Reads and drafts must survive both signup and publishing kill
switches.

`/.well-known/did.json` on the PDS hostname is not required for this OAuth and
repository-write design.
