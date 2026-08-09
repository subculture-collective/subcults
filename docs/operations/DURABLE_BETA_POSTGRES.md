# Durable Beta PostgreSQL Operations

This is the runtime contract for the public-beta API. PostgreSQL/PostGIS is the
only production persistence mode. `SUBCULT_IN_MEMORY_REPOSITORIES=true` is a
local fixture mode and must not be set in a release environment.

## Durable beta surface

The production runtime persists Scenes, Events, RSVPs, Places, Profiles, Acts,
Tours, Appearances, sources/assertions, audience contacts/links/relationships,
consent/suppressions, Signals/revisions/deliveries, identity, protected-location
grants, and browser push subscriptions.

The following older subsystems remain memory-backed and are explicitly returned
as `501 feature_disabled_beta` in the durable runtime: memberships, posts and
feeds, streams/LiveKit, alliances, trust, payments/Stripe, uploads, account
export/deletion, and telemetry intake. They must not be enabled until their SQL
adapters and restart tests exist.

## Local startup

```bash
go run ./cmd/generate-local-secrets
docker compose up -d postgres
DATABASE_URL='postgres://subcults:subcults-dev-password@127.0.0.1:5439/subcults?sslmode=disable' \
  MIGRATE_USE_DOCKER=1 ./scripts/migrate.sh up
docker compose up -d --build api
curl --fail http://127.0.0.1:8080/health/ready
```

The generator refuses to overwrite `deploy/.env`, writes it with mode `0600`,
and does not print secret values. The VAPID public value is copied to the Vite
build variable in the same file.

## Data guarantees

- Scene/Event adapters clear precise coordinates when retention consent is
  false. Event discovery additionally projects precise coordinates only for
  public events; protected coordinates remain available solely to the explicit
  authorization path.
- Audience delivery authorization resolves verified contact state, effective
  consent, and active suppressions in one repeatable-read transaction under a
  per-contact advisory lock. The dispatcher checks it immediately before send.
- Signal revisions are append-only in the repository and protected by database
  triggers that reject `UPDATE` and `DELETE`.
- Source observations are deduplicated without deleting assertion history;
  corrections link to one retained superseded assertion.
- Scene, Event, Place, Profile, Tour, and Appearance updates compare the supplied
  version and return `409 Conflict` for stale Studio writes.

## Pooling and timeouts

Defaults are 20 open connections, 5 idle connections, 30-minute maximum
lifetime, and 5-minute maximum idle time. Every new PostgreSQL connection gets
a 5-second statement timeout, 2-second lock timeout, and 10-second idle
transaction timeout. Override these through the `DB_*` values documented in
`deploy/.env.example`.

Readiness performs a live database ping with a five-second request deadline. A
database outage returns `503` and readiness recovers after PostgreSQL returns.
SIGTERM gives the HTTP server and tracing provider ten seconds to shut down,
then closes the SQL pool.

## Migrations and failure behavior

The API requires schema version 40 and exits before binding its port when the
schema is older or inaccessible. Deployment must run migrations before
recreating the API. Validate both paths before release:

1. apply migrations 0 through 40 to an empty PostGIS database;
2. apply through 39, then upgrade that populated schema to 40;
3. start the API against version 39 and confirm a non-zero exit;
4. run the tagged database integration tests against version 40.

```bash
DATABASE_URL="$TEST_DATABASE_URL" TEST_DATABASE_URL="$TEST_DATABASE_URL" \
  go test -tags=integration ./internal/db -count=1
```

## Backup and restore verification

Use a PostgreSQL client with the same server minor version. For the local
Compose database, use the server container itself:

```bash
DATABASE_URL='postgres://subcults:subcults-dev-password@127.0.0.1:5439/subcults?sslmode=disable' \
BACKUP_USE_DOCKER=1 \
BACKUP_DOCKER_CONTAINER=subcults-postgres \
BACKUP_DATABASE_URL='postgres://subcults:subcults-dev-password@127.0.0.1:5432/subcults?sslmode=disable' \
./scripts/backup.sh
```

A backup is not qualified until it restores into a separate empty database and
the restored schema reports version 40. Managed production also needs its
provider snapshot/PITR policy, retention, access controls, and a scheduled
restore drill; a successful local dump is not evidence for those controls.
