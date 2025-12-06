# Privacy Technical Overview

Subcult implements a **privacy-first** architecture designed to protect underground music communities. This document describes the technical privacy measures currently implemented and planned.

> **Note:** This is a technical summary for contributors and community review—not legal terms of service.

## Location Handling

Location data is handled with explicit consent controls and default privacy protections.

### Consent Model

All location-aware entities (Scenes, Events) include an `allow_precise` consent flag:

- **Default:** `allow_precise = FALSE` — precise coordinates are never stored without explicit opt-in
- **Database Enforcement:** Schema constraints ensure `precise_point IS NULL` when `allow_precise = FALSE`
- **Application Enforcement:** The `EnforceLocationConsent()` method clears precise coordinates before any persistence operation

```sql
-- Database constraint (from migrations/000000_initial_schema.up.sql)
CONSTRAINT chk_precise_consent CHECK (
    allow_precise = TRUE OR precise_point IS NULL
)
```

### Coarse Geohash

When precise location consent is not granted, only coarse location is stored:

- **Precision:** 6-character geohash (~±0.61 km accuracy)
- **Purpose:** Enables regional discovery without pinpointing exact venues
- **Validation:** Only valid base32 geohash characters are accepted; invalid input is rejected

The `RoundGeohash()` function truncates geohashes to the configured precision, preventing accidental leakage of higher-precision data.

### Location Jitter (Planned)

Future enhancement: random offset applied to map display coordinates to prevent deanonymization through coordinate triangulation.

## Media Sanitization

### EXIF Stripping

All uploaded media has EXIF metadata stripped before storage to prevent location/device leakage:

- **GPS coordinates** embedded in photos
- **Device identifiers** (camera make, model, serial numbers)
- **Timestamps** (original capture time, modification time)
- **Camera metadata** (exposure, ISO, aperture, software)

**Implementation**: The `internal/image` package uses [bimg](https://github.com/h2non/bimg) (libvips binding) to:
1. Strip all EXIF metadata with `StripMetadata: true`
2. Re-encode images to JPEG, WebP, or PNG formats
3. Apply EXIF orientation correction before stripping (ensures correct display)
4. Maintain image quality with configurable settings (default: 85%)

**Usage**:
```go
import "github.com/onnwee/subcults/internal/image"

// Process with defaults (JPEG, quality 85, strip metadata)
sanitizedBytes, err := image.Process(fileReader)

// Verify EXIF was removed
noEXIF, err := image.VerifyNoEXIF(sanitizedBytes)
```

**Status**: ✅ Implemented (service layer). Integration with media upload API endpoints is in progress.

See [`internal/image/README.md`](../internal/image/README.md) for detailed documentation and configuration options.

### Storage Security

Media assets are stored in Cloudflare R2 with:

- No public listing of buckets
- Signed URLs for time-limited access (planned)
- Content-type validation to prevent MIME-type attacks (planned)

## Access Logging

Structured request logging captures security-relevant events without excessive personal data.

### Logged Fields

| Field | Description |
|-------|-------------|
| `request_id` | UUID for request correlation (from `X-Request-ID` header or auto-generated) |
| `method` | HTTP method |
| `path` | Request path (no query strings logged) |
| `status` | HTTP response status code |
| `latency_ms` | Request duration in milliseconds |
| `size` | Response body size |
| `user_did` | Authenticated user's DID (if present) |
| `error_code` | Application error code (for 4xx/5xx responses) |

### What Is NOT Logged

- Request bodies or form data
- Full URLs with query parameters
- IP addresses (except for rate limiting decisions)
- Authentication credentials

### Log Levels

- **5xx errors:** `ERROR` level
- **4xx errors:** `WARN` level
- **Success:** `INFO` level

## User Controls

### Authentication

JWT-based authentication with Decentralized Identifiers (DIDs):

- **Access tokens:** 15-minute expiry, includes `did` claim
- **Refresh tokens:** 7-day expiry, no DID (reduces exposure)
- **Algorithm:** HS256 with validated signing method
- **Leeway:** 30-second clock skew tolerance

### Rate Limiting

Tiered rate limits protect against abuse:

| Scope | Limit | Window |
|-------|-------|--------|
| Global | 100 requests | 1 minute |
| Auth endpoints | 10 requests | 1 minute |
| Search endpoints | 30 requests | 1 minute |

Rate limit keys use:
1. Authenticated user DID (preferred)
2. IP address from `X-Forwarded-For`, `X-Real-IP`, or connection (fallback)

Standard headers returned on rate limit:
- `Retry-After`: Seconds until limit resets
- `X-RateLimit-Reset`: Unix timestamp when limit resets

### Request ID Validation

Incoming `X-Request-ID` headers are validated:
- Maximum 128 characters
- Alphanumeric, hyphens, and underscores only
- Invalid IDs are replaced with generated UUIDs

This prevents log injection attacks while supporting request correlation.

## Data Retention

> **Placeholder:** Retention policies are under development. The following are planned guidelines:

| Data Type | Retention Period | Notes |
|-----------|------------------|-------|
| Access logs | 90 days | Security audit trail |
| Soft-deleted content | 30 days | Grace period before hard delete |
| Session tokens | Until expiry | Access: 15min, Refresh: 7 days |
| Uploaded media | Until user deletion | Subject to storage quotas |

## Decentralized Identity

Subcult uses [AT Protocol](https://atproto.com/) for decentralized identity:

- User identity is tied to DIDs, not platform accounts
- Data portability through Jetstream ingestion
- No centralized identity provider lock-in

## Future Enhancements

Privacy improvements tracked in the [Privacy & Safety Epic](https://github.com/subculture-collective/subcults/issues/6):

- [x] EXIF/metadata stripping for uploaded media (service layer implemented)
- [ ] EXIF stripping integration with media upload API endpoints
- [ ] Location jitter for map display
- [ ] Signed URLs for media access
- [ ] Configurable data export (GDPR-style)
- [ ] Trust graph privacy controls (alliance visibility)
- [ ] Content encryption for private scenes

## Reporting Issues

Found a privacy concern? Please report responsibly:

1. **Security vulnerabilities:** See `SECURITY.md` (when available) or contact maintainers directly
2. **General privacy feedback:** Open a GitHub issue with the `privacy` label

---

*Last updated: December 2024*
