# Self-hosted Redis deployment contract

## Goal

Give Subcults a private, persistent Redis backend for distributed rate limits
without coupling it to another application's datastore or exposing Redis on a
host port.

## Placement

- Local development: the root Compose project, on its private default network.
- NUC staging: `/srv/server/projects/subcults`, on the existing external
  `projects` network beside the Subcults API.
- Image: `redis:7-alpine`, pinned to the Redis 7 major line.
- State: a dedicated `subcults_redis_data` Docker volume with AOF enabled.
- Public routes and host ports: none.

The NUC service requires a generated password because the `projects` network is
shared. `REDIS_URL` and `REDIS_PASSWORD` stay in the deployment `.env` and must
not enter Git. Local development may use the isolated passwordless service.

## Safety and rollback

Provisioning Redis does not alter PostgreSQL, Caddy, DNS, or firewall rules.
The application fails startup when a configured Redis cannot be reached. To
roll back, remove `REDIS_URL`, recreate only the API, and then remove the Redis
service after confirming the API has returned to its single-instance in-memory
rate-limit store. Preserve the Redis volume until rollback is accepted.

## Verification

1. `docker compose config --quiet` succeeds.
2. The Redis container becomes healthy without publishing a host port.
3. An authenticated `PING` returns `PONG` from inside the container.
4. API logs report `rate limiting initialized with Redis backend`.
5. `/health/ready` reports both `db` and `redis` as `ok`.
6. An API recreation does not remove the Redis volume or its AOF state.

## PostgreSQL boundary

The NUC already provides `postgis/postgis:16-3.4-alpine`. The Subcults database
has PostGIS and PostGIS Topology 3.4.3 installed. Database migration and API
release remain a separate operation: the live NUC database must reach schema
version 40 before the durable API can replace the currently deployed build.
