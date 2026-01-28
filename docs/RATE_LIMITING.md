# Rate Limiting

The Subcults API implements comprehensive rate limiting to protect against abuse and ensure fair resource allocation.

## Features

- **Redis-backed distributed rate limiting** for multi-instance deployments
- **Per-endpoint limits** with different quotas for different operations
- **Per-user and per-IP limits** with automatic fallback
- **Fail-open design** - if Redis is unavailable, requests are allowed to prevent outages
- **Prometheus metrics** for monitoring rate limit violations
- **Structured logging** for all rate limit events

## Configuration

### Redis Connection (Optional)

To enable distributed rate limiting with Redis, set the `REDIS_URL` environment variable:

```bash
REDIS_URL="redis://localhost:6379"
# or with password
REDIS_URL="redis://:password@localhost:6379/0"
```

If `REDIS_URL` is not set, the system will use in-memory rate limiting (suitable only for single-instance deployments).

### Rate Limits

The following rate limits are enforced:

| Endpoint | Limit | Window | Key Type |
|----------|-------|--------|----------|
| Search endpoints (`/search/*`) | 100 requests | 1 minute | Per user (authenticated) or IP |
| Stream join (`/streams/{id}/join`) | 10 requests | 1 minute | Per user |
| Event creation (`POST /events`) | 5 requests | 1 hour | Per user |
| General (all other endpoints) | 1000 requests | 1 minute | Per IP |

### Rate Limit Headers

All responses include rate limit headers:

- `X-RateLimit-Limit`: Maximum number of requests allowed in the window
- `X-RateLimit-Remaining`: Number of requests remaining in current window
- `X-RateLimit-Reset`: Unix timestamp when the limit resets (included in 429 responses)
- `Retry-After`: Seconds to wait before retrying (included in 429 responses)

### Example Response

**Normal Request:**
```
HTTP/1.1 200 OK
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
Content-Type: application/json
...
```

**Rate Limited Request:**
```
HTTP/1.1 429 Too Many Requests
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 0
X-RateLimit-Reset: 1706476800
Retry-After: 45
Content-Type: text/plain

Too Many Requests
```

## Monitoring

### Metrics

Rate limiting metrics are exposed on the `/metrics` endpoint (Prometheus format):

- `rate_limit_requests_total{endpoint, key_type}` - Total number of rate limit checks
- `rate_limit_blocked_total{endpoint, key_type}` - Total number of blocked requests

Labels:
- `endpoint`: The API endpoint path (e.g., `/search/events`)
- `key_type`: Either `user` (authenticated) or `ip` (anonymous)

### Logs

Rate limit violations are logged with structured fields:

```json
{
  "level": "warn",
  "method": "GET",
  "path": "/search/events",
  "status": 429,
  "latency_ms": 2,
  "error_code": "rate_limit_exceeded",
  "rate_limit_key": "user:did:web:example.com:user123",
  "message": "request completed"
}
```

## Implementation Details

### Algorithm

The system uses a **sliding window counter** algorithm for accurate rate limiting:

1. Each request increments a counter for the key (user ID or IP)
2. Old entries outside the time window are automatically removed
3. If the counter exceeds the limit, the request is blocked
4. The window slides continuously, providing smooth rate limiting

### Fail-Open Design

If Redis becomes unavailable:
- All requests are **allowed** to prevent cascading failures
- Full quota is returned in headers
- Errors are logged for monitoring

This ensures high availability while maintaining observability.

### Key Types

- **User keys** (`user:{did}`): Used for authenticated requests
- **IP keys** (`ip:{address}`): Used for anonymous requests
- User keys take precedence when authentication is present

IP addresses are extracted from headers in this order:
1. `X-Forwarded-For` (first IP in chain)
2. `X-Real-IP`
3. `RemoteAddr`

## Development

### Running Tests

```bash
# Run all rate limiting tests
go test ./internal/middleware -v -run "TestRateLimit|TestMetrics"

# Run Redis integration tests (requires Redis on localhost:6379)
go test ./internal/middleware -v -run "TestRedis"
```

### In-Memory vs Redis

**In-Memory Store:**
- ✅ Zero dependencies
- ✅ Fast (no network latency)
- ❌ Not suitable for multi-instance deployments
- ❌ Limits lost on restart

**Redis Store:**
- ✅ Distributed rate limiting across instances
- ✅ Persistent limits across restarts
- ✅ Accurate sliding window implementation
- ❌ Requires Redis infrastructure
- ❌ Network latency overhead

## Troubleshooting

### "rate_limit_exceeded" errors in logs

This is expected behavior when users exceed their quota. Check:
1. Whether the limits are appropriate for your use case
2. If a specific user/IP is being abusive
3. Consider implementing admin bypass for trusted users

### Redis connection errors

If you see "failed to connect to Redis" errors:
1. Verify Redis is running and accessible
2. Check the `REDIS_URL` format
3. Ensure firewall rules allow connection
4. Consider using in-memory mode for development

### Limits not working as expected

1. Check that `REDIS_URL` is set correctly
2. Verify Redis is receiving commands (use `redis-cli monitor`)
3. Check metrics on `/metrics` endpoint
4. Review logs for `rate_limit_exceeded` events

## Future Enhancements

- [ ] Admin bypass based on user roles
- [ ] Dynamic rate limits based on user tier
- [ ] Rate limit configuration via API
- [ ] Per-organization rate limits
- [ ] Burst allowance with token bucket
