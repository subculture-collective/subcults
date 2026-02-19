# Subcults Security Threat Model (STRIDE Analysis)

**Version:** 1.0  
**Date:** February 2026  
**Status:** Active  
**Related Issues:** #308 (Security Hardening), #130 (Threat Model), #20 (Security Hardening)

---

## Executive Summary

This document provides a comprehensive STRIDE threat analysis for the Subcults platform—a privacy-first underground music community mapping system. The analysis identifies security threats across six categories (Spoofing, Tampering, Repudiation, Information Disclosure, Denial of Service, Elevation of Privilege), maps existing mitigations, scores risks, and provides actionable recommendations.

**Key Findings:**
- **Strengths:** Strong privacy controls, comprehensive audit logging, JWT-based authentication, rate limiting
- **Critical Gaps:** Missing JWT secret rotation mechanism, no Content Security Policy headers, file upload limits not evident
- **Overall Risk Level:** MEDIUM (with high-risk items requiring immediate attention)

---

## Table of Contents

1. [Threat Model Scope](#threat-model-scope)
2. [STRIDE Analysis](#stride-analysis)
   - [Spoofing (Identity Verification)](#1-spoofing-identity-verification)
   - [Tampering (Data Integrity)](#2-tampering-data-integrity)
   - [Repudiation (Non-Repudiation)](#3-repudiation-non-repudiation)
   - [Information Disclosure (Confidentiality)](#4-information-disclosure-confidentiality)
   - [Denial of Service (Availability)](#5-denial-of-service-availability)
   - [Elevation of Privilege (Authorization)](#6-elevation-of-privilege-authorization)
3. [Risk Scoring Matrix](#risk-scoring-matrix)
4. [Mitigation Mapping](#mitigation-mapping)
5. [Implementation Roadmap](#implementation-roadmap)
6. [Testing & Validation](#testing--validation)
7. [Stakeholder Review](#stakeholder-review)
8. [Appendices](#appendices)

---

## Threat Model Scope

### System Components

**Backend Services:**
- **API Service** (Go/chi): REST API for scenes, events, payments, auth, media
- **Indexer Service**: Jetstream consumer for AT Protocol data ingestion
- **Database**: Neon Postgres 16 with PostGIS

**Frontend:**
- React/TypeScript SPA with MapLibre
- Vite build system with SWC

**External Dependencies:**
- LiveKit Cloud (WebRTC streaming)
- Stripe Connect (payments)
- Cloudflare R2 (media storage)
- AT Protocol/Jetstream (decentralized identity)
- MapTiler (map tiles)
- Redis (rate limiting, optional)

### Trust Boundaries

1. **Public Internet ↔ API Server**: Primary attack surface
2. **API Server ↔ Database**: Requires secure connection
3. **API Server ↔ External Services**: Stripe, LiveKit, R2, Jetstream
4. **Frontend ↔ Backend API**: Client-server communication
5. **Indexer ↔ Jetstream Firehose**: Real-time data ingestion
6. **Users ↔ User-Generated Content**: Content validation required

### Assets to Protect

| Asset | Confidentiality | Integrity | Availability | Impact if Compromised |
|-------|----------------|-----------|--------------|----------------------|
| User DIDs | HIGH | CRITICAL | MEDIUM | Identity theft, impersonation |
| Location Data | CRITICAL | HIGH | MEDIUM | Privacy violation, physical security |
| Payment Data | CRITICAL | CRITICAL | HIGH | Financial fraud |
| JWT Secrets | CRITICAL | CRITICAL | HIGH | Full platform compromise |
| Database Credentials | CRITICAL | CRITICAL | HIGH | Data exfiltration, deletion |
| Audit Logs | MEDIUM | CRITICAL | HIGH | Loss of accountability |
| User Content | MEDIUM | HIGH | MEDIUM | Defacement, misinformation |

---

## STRIDE Analysis

### 1. Spoofing (Identity Verification)

**Definition:** An attacker impersonates a legitimate user or system component.

#### T-SPOOF-001: JWT Token Forgery
- **Threat:** Attacker creates fake JWT tokens to impersonate users
- **Attack Vector:** Weak signing secret, algorithm confusion attack (HS256 → none)
- **Impact:** CRITICAL - Full account takeover, unauthorized access to private scenes
- **Affected Assets:** User accounts, private scenes, payment data
- **Existing Mitigations:**
  - ✅ JWT tokens signed with HS256 using secret key (`JWT_SECRET`)
  - ✅ Algorithm validation enforced (`jwt.SigningMethodHS256.Alg()` check in `internal/auth/jwt.go:109`)
  - ✅ 15-minute access token expiry reduces exposure window
  - ✅ 30-second leeway for clock skew tolerance
  - ✅ Token validation in middleware
- **Risk Score:** **LOW** (well-mitigated)
- **Residual Risk:** Secret compromise due to weak key or leak
- **Recommendations:**
  - ✅ Already requires minimum 32-character secret
  - 🔴 **HIGH PRIORITY:** Implement secret rotation with dual-key support (see T-SPOOF-002)
  - ⚠️ Monitor for algorithm confusion attempts in logs
  - ⚠️ Consider migrating to RS256 (asymmetric) for better key management

#### T-SPOOF-002: JWT Secret Exposure
- **Threat:** JWT signing secret leaked through source control, logs, or error messages
- **Attack Vector:** Accidental commit to Git, logging in plaintext, server compromise
- **Impact:** CRITICAL - All tokens can be forged if secret is compromised
- **Affected Assets:** All user accounts, entire platform security
- **Existing Mitigations:**
  - ✅ Secret loaded from environment variable (`JWT_SECRET`)
  - ✅ No secrets in source code
  - ✅ `.env` files in `.gitignore`
  - ⚠️ No rotation mechanism visible
- **Risk Score:** **HIGH** (no rotation mechanism)
- **Residual Risk:** Once compromised, no way to invalidate old tokens without full outage
- **Recommendations:**
  - 🔴 **HIGH PRIORITY:** Implement dual JWT key support:
    ```go
    // Store two keys: JWT_SECRET_PRIMARY and JWT_SECRET_SECONDARY
    // Sign tokens with PRIMARY key
    // Validate tokens against BOTH keys (fallback to secondary)
    // Rotation process: Deploy with both keys → switch PRIMARY → remove old key
    ```
  - 🔴 Automate secret rotation every 90 days
  - ⚠️ Alert on JWT validation failures (potential compromise indicator)
  - ⚠️ Implement emergency secret invalidation procedure

#### T-SPOOF-003: Session Hijacking via Token Theft
- **Threat:** Attacker steals access or refresh token from client storage
- **Attack Vector:** XSS attack, localStorage theft, network interception (MITM)
- **Impact:** HIGH - Account takeover until token expires
- **Affected Assets:** User sessions, private data
- **Existing Mitigations:**
  - ✅ Access tokens expire in 15 minutes
  - ✅ Refresh tokens expire in 7 days
  - ✅ HTTPS enforced in production (assumed)
- **Risk Score:** **MEDIUM** (limited by token expiry)
- **Residual Risk:** 7-day window for refresh token abuse
- **Recommendations:**
  - ⚠️ Implement token revocation list (Redis-backed)
  - ⚠️ Use `HttpOnly` cookies instead of localStorage for tokens
  - ⚠️ Add `SameSite=Strict` cookie attribute
  - ⚠️ Implement token fingerprinting (bind to user-agent + IP hash)
  - ⚠️ Rotate refresh tokens on use (one-time use pattern)

#### T-SPOOF-004: Decentralized Identity (DID) Spoofing
- **Threat:** Attacker claims ownership of another user's DID
- **Attack Vector:** AT Protocol verification bypass, DID document manipulation
- **Impact:** HIGH - Impersonate users, post as others, access private scenes
- **Affected Assets:** User identities, content authorship, trust graph
- **Existing Mitigations:**
  - ✅ DIDs stored in JWT claims (`did` field)
  - ⚠️ **UNKNOWN:** DID verification process not evident in code review
- **Risk Score:** **HIGH** (insufficient verification visible)
- **Residual Risk:** Dependent on AT Protocol security
- **Recommendations:**
  - 🔴 **HIGH PRIORITY:** Document DID verification process
  - 🔴 Implement DID document signature verification
  - ⚠️ Validate DID ownership on every sensitive operation (not just JWT issuance)
  - ⚠️ Audit DID changes in audit logs
  - ⚠️ Implement DID challenge-response for high-value operations

#### T-SPOOF-005: API Request Forgery (CSRF)
- **Threat:** Attacker tricks user's browser into making unauthorized API requests
- **Attack Vector:** Cross-site request forgery via malicious website
- **Impact:** MEDIUM - Unauthorized actions (scene creation, membership changes)
- **Affected Assets:** User accounts, scenes, memberships
- **Existing Mitigations:**
  - ⚠️ **NOT FOUND:** No explicit CSRF protection visible in code
  - ✅ JWT tokens in headers (not cookies) provides partial protection
- **Risk Score:** **MEDIUM** (depends on token storage method)
- **Residual Risk:** If tokens stored in cookies without SameSite, vulnerable
- **Recommendations:**
  - ⚠️ If using cookies: Implement CSRF tokens for state-changing operations
  - ⚠️ Enforce `SameSite=Strict` on auth cookies
  - ⚠️ Validate `Referer`/`Origin` headers for sensitive endpoints
  - ⚠️ Implement double-submit cookie pattern if needed

---

### 2. Tampering (Data Integrity)

**Definition:** An attacker modifies data in transit or at rest.

#### T-TAMP-001: SQL Injection
- **Threat:** Attacker injects malicious SQL through user input
- **Attack Vector:** Unsanitized input in database queries
- **Impact:** CRITICAL - Database compromise, data exfiltration, data deletion
- **Affected Assets:** All database tables, PII, location data, payment info
- **Existing Mitigations:**
  - ✅ Repository pattern with parameterized queries (Go best practices)
  - ✅ Input validation in API handlers
  - ✅ ORM/query builder usage (assumed based on Go patterns)
- **Risk Score:** **LOW** (standard Go patterns prevent this)
- **Residual Risk:** Human error in raw SQL queries
- **Recommendations:**
  - ✅ Continue using parameterized queries for all database access
  - ⚠️ Automated SQL injection testing in CI/CD
  - ⚠️ Code review checklist: Flag raw SQL construction
  - ⚠️ Use static analysis tools (gosec, sqlvet)

#### T-TAMP-002: XSS (Cross-Site Scripting)
- **Threat:** Attacker injects malicious JavaScript into user-facing content
- **Attack Vector:** Scene descriptions, event names, post content, usernames
- **Impact:** HIGH - Session theft, malware delivery, phishing, defacement
- **Affected Assets:** User sessions, frontend integrity, user trust
- **Existing Mitigations:**
  - ✅ React's automatic XSS escaping (default behavior)
  - ⚠️ **PARTIAL:** Input validation in API handlers
  - ⚠️ **NOT FOUND:** No Content Security Policy (CSP) headers
- **Risk Score:** **MEDIUM** (React protects but no CSP defense-in-depth)
- **Residual Risk:** `dangerouslySetInnerHTML` usage, markdown rendering
- **Recommendations:**
  - 🔴 **HIGH PRIORITY:** Implement Content Security Policy headers:
    ```
    Content-Security-Policy: 
      default-src 'self'; 
      script-src 'self' https://trusted-cdn.com; 
      style-src 'self' 'unsafe-inline'; 
      img-src 'self' https://r2.cloudflarestorage.com data:; 
      connect-src 'self' wss://livekit.cloud wss://jetstream.atproto.com; 
      frame-ancestors 'none'; 
      base-uri 'self'; 
      form-action 'self'
    ```
  - ⚠️ Audit all `dangerouslySetInnerHTML` usage in frontend
  - ⚠️ Sanitize markdown rendering output (use DOMPurify)
  - ⚠️ Add automated XSS testing to E2E suite
  - ⚠️ Implement input sanitization for rich text fields

#### T-TAMP-003: Input Validation Bypass
- **Threat:** Attacker submits malformed data to bypass business logic
- **Attack Vector:** Crafted JSON payloads, oversized fields, invalid types
- **Impact:** MEDIUM - Data corruption, DoS, privilege escalation
- **Affected Assets:** Database integrity, application state
- **Existing Mitigations:**
  - ✅ Input validation in API handlers
  - ✅ Database constraints (e.g., `chk_precise_consent`)
  - ✅ Request ID validation (max 128 chars, alphanumeric)
  - ✅ Type validation via Go structs
- **Risk Score:** **LOW** (multi-layered validation)
- **Residual Risk:** Inconsistent validation across endpoints
- **Recommendations:**
  - ⚠️ Centralize validation logic in reusable validator package
  - ⚠️ Add JSON schema validation for all POST/PUT endpoints
  - ⚠️ Implement max request body size limits (e.g., 10MB)
  - ⚠️ Log validation failures for monitoring and attack detection

#### T-TAMP-004: Location Data Manipulation
- **Threat:** Attacker bypasses location consent checks to expose precise coordinates
- **Attack Vector:** Direct database access, API parameter manipulation
- **Impact:** HIGH - Privacy violation, GDPR breach, user trust loss
- **Affected Assets:** User location data, scene/event precise coordinates
- **Existing Mitigations:**
  - ✅ Database constraint: `allow_precise = TRUE OR precise_point IS NULL`
  - ✅ `EnforceLocationConsent()` method in models (`internal/scene/model.go`)
  - ✅ Repository-level enforcement (`internal/scene/repository.go`)
  - ✅ Geohash jitter for non-consented locations (`internal/geo/jitter.go`)
  - ✅ Privacy tests (`internal/scene/privacy_test.go`)
- **Risk Score:** **LOW** (excellent multi-layer protection)
- **Residual Risk:** Human error (forgetting to call `EnforceLocationConsent()`)
- **Recommendations:**
  - ✅ Already excellently protected with database constraints
  - ⚠️ Add pre-commit hook to flag missing consent enforcement
  - ⚠️ Automated tests for privacy invariants (already exists)
  - ⚠️ Regular privacy audits: Query for constraint violations

#### T-TAMP-005: Audit Log Tampering
- **Threat:** Attacker modifies or deletes audit logs to hide malicious activity
- **Attack Vector:** Direct database access, SQL injection, insider threat
- **Impact:** CRITICAL - Loss of audit trail, compliance violation, inability to investigate incidents
- **Affected Assets:** Audit logs, compliance status, incident response capability
- **Existing Mitigations:**
  - ✅ Audit logs in separate table (`audit_logs`)
  - ✅ Indexed for efficient querying
  - ⚠️ **NOT FOUND:** No append-only enforcement or hash chain
- **Risk Score:** **MEDIUM** (logs can be deleted by database admin)
- **Residual Risk:** Database admin can modify logs without detection
- **Recommendations:**
  - ⚠️ Implement append-only log table (revoke UPDATE/DELETE permissions)
  - ⚠️ Add hash chain for tamper detection:
    ```sql
    ALTER TABLE audit_logs ADD COLUMN previous_log_hash VARCHAR(64);
    -- Compute: current_hash = SHA256(previous_hash || log_data)
    ```
  - ⚠️ Export logs to immutable storage (S3 Glacier, WORM storage) daily
  - ⚠️ Implement log integrity verification script (run nightly)
  - ⚠️ Alert on hash chain breaks

#### T-TAMP-006: File Upload Malware
- **Threat:** Attacker uploads malicious files disguised as media
- **Attack Vector:** Crafted file extensions, MIME type mismatch, polyglot files
- **Impact:** HIGH - Malware distribution, server compromise, client-side attacks
- **Affected Assets:** R2 storage, user devices, platform reputation
- **Existing Mitigations:**
  - ✅ EXIF metadata stripping (`internal/image/processor.go`)
  - ⚠️ **PARTIAL:** Content-type validation (implementation needs verification)
- **Risk Score:** **MEDIUM** (partial mitigations)
- **Residual Risk:** Non-image files, sophisticated polyglot attacks
- **Recommendations:**
  - ⚠️ Whitelist allowed MIME types (images: JPEG, PNG, WebP; audio: MP3, AAC, Opus)
  - ⚠️ Verify file magic bytes match declared MIME type
  - ⚠️ Scan uploads with antivirus (ClamAV or cloud service)
  - ⚠️ Serve user-uploaded content from separate domain (e.g., `media.subcults.com`)
  - ⚠️ Set `Content-Disposition: attachment` for downloads
  - ⚠️ Implement file size limits per type

---

### 3. Repudiation (Non-Repudiation)

**Definition:** An attacker denies performing an action without proof to the contrary.

#### T-REPUD-001: Insufficient Audit Logging
- **Threat:** Actions are not logged, allowing attackers to deny responsibility
- **Attack Vector:** Missing log entries for sensitive operations
- **Impact:** HIGH - Cannot investigate incidents, compliance failure, legal liability
- **Affected Assets:** Incident response, compliance, legal defense
- **Existing Mitigations:**
  - ✅ Comprehensive audit logging system (`internal/audit/`)
  - ✅ Logs: user DID, entity type/ID, action, timestamp, request ID, IP, user agent
  - ✅ Actions tracked: `access_precise_location`, `view_admin_panel`, etc.
  - ✅ Indexed for efficient querying
  - ✅ Fail-closed design (request fails if audit logging fails)
- **Risk Score:** **LOW** (comprehensive system in place)
- **Residual Risk:** Gaps in coverage (not all endpoints instrumented)
- **Recommendations:**
  - ⚠️ Audit logging coverage report: Document which endpoints have audit logs
  - ⚠️ Add audit logs to all state-changing operations:
    - Scene/event creation/deletion
    - Membership changes
    - Payment transactions
    - Alliance formation
    - Privacy setting changes
  - ⚠️ Automated test: Verify audit log entry for each sensitive endpoint

#### T-REPUD-002: Request Logging Gaps
- **Threat:** Attacker's malicious requests are not logged for forensic analysis
- **Attack Vector:** Logging disabled, log level too high, incomplete request data
- **Impact:** MEDIUM - Cannot trace attacker actions, poor incident response
- **Affected Assets:** Security monitoring, incident response
- **Existing Mitigations:**
  - ✅ Structured request logging (`internal/middleware/logging.go`)
  - ✅ Fields: request_id, method, path, status, latency, user_did, error_code
  - ✅ PII-aware: No request bodies, query params, or full URLs logged
  - ✅ Environment-specific log levels (debug in dev, info in prod)
- **Risk Score:** **LOW** (excellent structured logging)
- **Residual Risk:** Logs may be too verbose for long-term storage
- **Recommendations:**
  - ⚠️ Implement log retention policy:
    - Access logs: 90 days
    - Error logs: 30 days
    - Audit logs: 7 years (compliance requirement)
  - ⚠️ Aggregate logs to centralized system (Loki, Elasticsearch, Datadog)
  - ⚠️ Set up alerts for suspicious patterns:
    - High rate of 401/403 errors (>10/min from single IP)
    - Repeated rate limiting (429 responses)
    - Failed JWT validation attempts
    - Unusual geographic access patterns

#### T-REPUD-003: Timestamp Manipulation
- **Threat:** Attacker modifies system time to forge log timestamps
- **Attack Vector:** Compromised server, NTP manipulation, database time tampering
- **Impact:** MEDIUM - Cannot establish accurate timeline of events
- **Affected Assets:** Audit trail integrity, incident investigation
- **Existing Mitigations:**
  - ✅ Timestamps use `TIMESTAMPTZ` (timezone-aware)
  - ✅ Database-generated timestamps (`DEFAULT NOW()`)
  - ⚠️ NTP configuration not visible
- **Risk Score:** **LOW** (database-controlled timestamps)
- **Residual Risk:** Server compromise allows time manipulation
- **Recommendations:**
  - ⚠️ Use NTP time synchronization with authenticated time servers
  - ⚠️ Alert on significant time drift (>1 second)
  - ⚠️ Add hash chain to audit logs (links timestamps cryptographically)
  - ⚠️ Include multiple time sources in critical logs (server time + external timestamp service)

---

### 4. Information Disclosure (Confidentiality)

**Definition:** An attacker gains unauthorized access to sensitive information.

#### T-INFO-001: Precise Location Exposure Without Consent
- **Threat:** User location revealed without explicit consent
- **Attack Vector:** API endpoint returns precise coordinates when `allow_precise=false`
- **Impact:** CRITICAL - Privacy violation, GDPR breach, physical security risk
- **Affected Assets:** User privacy, scene/event locations, platform trust
- **Existing Mitigations:**
  - ✅ Database constraint: `allow_precise = TRUE OR precise_point IS NULL`
  - ✅ `EnforceLocationConsent()` automatic enforcement
  - ✅ Repository-level checks before returning data
  - ✅ Geohash jitter for public coordinates (6-char ~±0.61km)
  - ✅ Comprehensive privacy tests
- **Risk Score:** **LOW** (excellent multi-layer protection)
- **Residual Risk:** Human error, edge cases in new features
- **Recommendations:**
  - ✅ Already excellently protected
  - ⚠️ Regular privacy audits: Query for violations
  - ⚠️ Red team exercise: Attempt to bypass privacy controls
  - ⚠️ Privacy impact assessment for all new location-related features

#### T-INFO-002: PII Leakage in Logs
- **Threat:** Sensitive data logged in plaintext (DIDs, IPs, user agents)
- **Attack Vector:** Log aggregation, log file access, log exfiltration
- **Impact:** HIGH - GDPR violation, user privacy breach, data subject access request complexity
- **Affected Assets:** User privacy, compliance status
- **Existing Mitigations:**
  - ✅ PII-aware logging: No request bodies, query params, or full URLs
  - ✅ Audit logging explicitly documents PII storage (`internal/audit/README.md`)
  - ⚠️ **PARTIAL:** DIDs, IPs, and user agents ARE logged (necessary for security)
- **Risk Score:** **MEDIUM** (legitimate use but creates risk)
- **Residual Risk:** Log access by unauthorized personnel, retention too long
- **Recommendations:**
  - ⚠️ Implement log access controls: Only security/compliance teams
  - ⚠️ Pseudonymize DIDs in logs: Hash with daily rotating salt
  - ⚠️ Truncate IP addresses: Store /24 for IPv4, /48 for IPv6
  - ⚠️ Document log retention policy in privacy policy
  - ⚠️ Implement user data export (GDPR Article 15) including log entries

#### T-INFO-003: EXIF Metadata Leakage
- **Threat:** Uploaded photos contain GPS coordinates, device info, timestamps
- **Attack Vector:** User uploads photo with embedded EXIF data
- **Impact:** HIGH - User location deanonymization, privacy violation
- **Affected Assets:** User privacy, platform trust
- **Existing Mitigations:**
  - ✅ EXIF stripping implemented (`internal/image/processor.go`)
  - ✅ Uses libvips (bimg) with `StripMetadata: true`
  - ✅ EXIF orientation correction before stripping
  - ✅ Verification function (`image.VerifyNoEXIF()`)
- **Risk Score:** **LOW** (excellent protection)
- **Residual Risk:** Integration gap (API endpoints may not call processor)
- **Recommendations:**
  - 🔴 **HIGH PRIORITY:** Integrate EXIF stripping with upload API (`POST /api/upload`)
  - ⚠️ Verify EXIF removal with automated tests
  - ⚠️ Process ALL image uploads (scene/event images, profile pictures, etc.)
  - ⚠️ Add monitoring: Alert if images with EXIF detected in storage

#### T-INFO-004: Error Message Information Leakage
- **Threat:** Detailed error messages reveal system internals
- **Attack Vector:** Malformed requests trigger verbose error responses
- **Impact:** MEDIUM - System enumeration, vulnerability discovery, attack surface mapping
- **Affected Assets:** System internals, database schema, file paths
- **Existing Mitigations:**
  - ✅ Structured error logging with error codes
  - ✅ Generic error responses to clients (e.g., "Internal Server Error")
  - ⚠️ **NEEDS VERIFICATION:** Stack traces not returned in production
- **Risk Score:** **LOW** (good error handling practices)
- **Residual Risk:** Debug mode enabled in production, unhandled exceptions
- **Recommendations:**
  - ⚠️ Verify stack traces never returned in production responses
  - ⚠️ Implement error response sanitization middleware
  - ⚠️ Log detailed errors server-side, return generic messages to clients
  - ⚠️ Differentiate error messages by environment (verbose in dev, generic in prod)
  - ⚠️ Custom error pages for 404, 500 (no framework details)

#### T-INFO-005: Scene Visibility Bypass
- **Threat:** Attacker accesses members-only or hidden scenes without authorization
- **Attack Vector:** API endpoint doesn't enforce visibility checks, direct ID enumeration
- **Impact:** HIGH - Privacy violation, unauthorized access to private communities
- **Affected Assets:** Private scenes, membership data, user trust
- **Existing Mitigations:**
  - ✅ Three visibility modes: public, members-only, hidden
  - ✅ Hidden scenes excluded from search results
  - ✅ Authorization checks return same 404 as non-existent scenes (prevents enumeration)
  - ✅ Membership status checked (`active` required, not `pending`/`rejected`)
  - ✅ Visibility enforcement in handlers
- **Risk Score:** **LOW** (comprehensive visibility system)
- **Residual Risk:** Inconsistent enforcement across API endpoints
- **Recommendations:**
  - ⚠️ Centralize visibility checks in authorization middleware
  - ⚠️ Audit all scene/event endpoints for visibility enforcement
  - ⚠️ Automated tests for each visibility mode + authorization combination
  - ⚠️ Rate limit scene ID enumeration attempts

#### T-INFO-006: Database Credential Exposure
- **Threat:** Database connection string leaked in logs, error messages, source control
- **Attack Vector:** Misconfigured logging, committed `.env` file, error stack traces
- **Impact:** CRITICAL - Full database compromise, data exfiltration, data deletion
- **Affected Assets:** Entire database, all user data, platform integrity
- **Existing Mitigations:**
  - ✅ Credentials loaded from environment variables
  - ✅ `.env` files in `.gitignore`
  - ✅ No credentials in source code
- **Risk Score:** **MEDIUM** (depends on operational security)
- **Residual Risk:** Accidental commit, log file exposure, server compromise
- **Recommendations:**
  - ⚠️ Use secret management service (AWS Secrets Manager, HashiCorp Vault)
  - ⚠️ Implement pre-commit hooks to detect credential patterns
  - ⚠️ Rotate database credentials every 90 days
  - ⚠️ Use least-privilege database users (read-only for indexer, etc.)
  - ⚠️ Enable database connection encryption (SSL/TLS)
  - ⚠️ Monitor for failed authentication attempts to database

#### T-INFO-007: Third-Party API Key Exposure
- **Threat:** Stripe, LiveKit, R2, or MapTiler API keys leaked
- **Attack Vector:** Client-side code, source control, logs, error messages
- **Impact:** HIGH - Financial loss, service abuse, quota exhaustion
- **Affected Assets:** Payment processing, streaming, storage, map tiles
- **Existing Mitigations:**
  - ✅ Server-side API key usage (not exposed to frontend)
  - ✅ Keys loaded from environment variables
  - ⚠️ MapTiler key may be used in frontend (visible to clients)
- **Risk Score:** **MEDIUM** (operational risk)
- **Residual Risk:** Keys in logs, accidental exposure, abuse
- **Recommendations:**
  - ⚠️ Implement API key rotation schedule:
    - Stripe: 90 days
    - LiveKit: 90 days
    - R2: 180 days
    - MapTiler: Proxy through backend or use referrer restrictions
  - ⚠️ Monitor API usage for anomalies
  - ⚠️ Set up alerts for quota approaching limits
  - ⚠️ Use separate keys for dev/staging/production

---

### 5. Denial of Service (Availability)

**Definition:** An attacker makes the system unavailable to legitimate users.

#### T-DOS-001: Rate Limiting Bypass
- **Threat:** Attacker exceeds rate limits through IP rotation, distributed requests
- **Attack Vector:** Botnet, proxy rotation, distributed attack
- **Impact:** MEDIUM - Service degradation, resource exhaustion
- **Affected Assets:** API availability, user experience, infrastructure costs
- **Existing Mitigations:**
  - ✅ Redis-backed distributed rate limiting
  - ✅ Per-endpoint limits (search: 30/min, auth: 10/min, global: 100/min)
  - ✅ Per-user (DID) and per-IP limiting
  - ✅ Fail-open design (allows requests if Redis unavailable)
  - ✅ Prometheus metrics for rate limit violations
- **Risk Score:** **MEDIUM** (distributed attacks can bypass)
- **Residual Risk:** No IP reputation, no CAPTCHA for anonymous users
- **Recommendations:**
  - ⚠️ Implement adaptive rate limiting based on user reputation
  - ⚠️ Add CAPTCHA challenge for repeated violations
  - ⚠️ Use CDN/WAF with DDoS protection (Cloudflare, AWS Shield)
  - ⚠️ Monitor rate limit violations for attack patterns
  - ⚠️ Implement IP reputation scoring

#### T-DOS-002: Resource Exhaustion via Large Uploads
- **Threat:** Attacker uploads extremely large files to exhaust storage/bandwidth
- **Attack Vector:** Large file uploads, repeated upload requests
- **Impact:** MEDIUM - Storage costs, bandwidth exhaustion, slow service
- **Affected Assets:** R2 storage quota, bandwidth, upload performance
- **Existing Mitigations:**
  - ⚠️ **NOT EVIDENT:** Max file size limits not found in code review
  - ⚠️ **NOT EVIDENT:** Upload quota per user not found
- **Risk Score:** **HIGH** (no visible limits)
- **Residual Risk:** Unlimited storage consumption
- **Recommendations:**
  - 🔴 **HIGH PRIORITY:** Implement max file size limits:
    - Images: 10MB
    - Audio: 50MB
    - Video: 500MB
  - 🔴 Implement per-user storage quota (e.g., 1GB free, 10GB paid)
  - ⚠️ Add request body size limit in middleware (e.g., 100MB max)
  - ⚠️ Implement upload rate limiting (separate from API rate limits)
  - ⚠️ Monitor R2 storage usage with alerts

#### T-DOS-003: Slowloris / Slow HTTP Attacks
- **Threat:** Attacker opens many connections and sends data slowly to exhaust server resources
- **Attack Vector:** Partial HTTP requests, slow POST bodies
- **Impact:** HIGH - Server connection exhaustion, service unavailability
- **Affected Assets:** API server availability
- **Existing Mitigations:**
  - ⚠️ **UNKNOWN:** Reverse proxy (Caddy) timeout configuration not visible
  - ⚠️ **UNKNOWN:** Go http.Server timeout settings not evident
- **Risk Score:** **MEDIUM** (depends on configuration)
- **Residual Risk:** No visible timeout protection
- **Recommendations:**
  - ⚠️ Configure Caddy timeouts:
    ```
    timeouts {
      read_body 30s
      read_header 10s
      write 60s
      idle 120s
    }
    ```
  - ⚠️ Configure Go http.Server timeouts:
    ```go
    server := &http.Server{
      ReadTimeout:  10 * time.Second,
      WriteTimeout: 60 * time.Second,
      IdleTimeout:  120 * time.Second,
    }
    ```
  - ⚠️ Use reverse proxy (Caddy/nginx) with connection limits

#### T-DOS-004: Database Connection Pool Exhaustion
- **Threat:** Attacker exhausts database connections through repeated requests
- **Attack Vector:** High-frequency requests, long-running queries
- **Impact:** HIGH - Database unavailable, cascading service failure
- **Affected Assets:** Database availability, API service
- **Existing Mitigations:**
  - ⚠️ **UNKNOWN:** Connection pool limits not visible in code review
- **Risk Score:** **MEDIUM** (standard Go SQL practices likely in use)
- **Residual Risk:** No visible connection limits
- **Recommendations:**
  - ⚠️ Configure database connection pool:
    ```go
    db.SetMaxOpenConns(25) // Max concurrent connections
    db.SetMaxIdleConns(5)  // Max idle connections
    db.SetConnMaxLifetime(5 * time.Minute)
    db.SetConnMaxIdleTime(1 * time.Minute)
    ```
  - ⚠️ Implement query timeout context (e.g., 30 seconds)
  - ⚠️ Monitor database connection usage
  - ⚠️ Optimize slow queries (use EXPLAIN ANALYZE)

#### T-DOS-005: Jetstream Reconnection Storm
- **Threat:** Indexer service repeatedly reconnects to Jetstream, overwhelming the service
- **Attack Vector:** Network instability, misconfigured reconnection logic
- **Impact:** LOW - Indexer unavailability, data ingestion delay
- **Affected Assets:** Indexer service, real-time data feed
- **Existing Mitigations:**
  - ✅ Exponential backoff for reconnection (observed in `docs/jetstream-reconnection.md`)
  - ✅ Backoff configuration documented
- **Risk Score:** **LOW** (backoff implemented)
- **Residual Risk:** Backoff tuning may be suboptimal
- **Recommendations:**
  - ⚠️ Review backoff parameters: Start 1s, max 5min, jitter ±20%
  - ⚠️ Add circuit breaker: Stop reconnecting after 10 consecutive failures
  - ⚠️ Alert on extended disconnection (>30 minutes)
  - ⚠️ Implement manual reconnection trigger for ops team

#### T-DOS-006: Regex Denial of Service (ReDoS)
- **Threat:** Malicious input triggers catastrophic regex backtracking
- **Attack Vector:** Crafted strings in scene names, descriptions, geohash validation
- **Impact:** MEDIUM - CPU exhaustion, request timeout, service degradation
- **Affected Assets:** API responsiveness
- **Existing Mitigations:**
  - ⚠️ **NEEDS REVIEW:** Regex usage in validation logic
  - ✅ Go regex engine has backtracking limits (built-in protection)
- **Risk Score:** **LOW** (Go regex engine has built-in protection)
- **Residual Risk:** Poorly written regex patterns
- **Recommendations:**
  - ⚠️ Audit all regex patterns for catastrophic backtracking
  - ⚠️ Use simple string operations instead of regex where possible
  - ⚠️ Set regex timeout (Go's `regexp` package has built-in protection)
  - ⚠️ Test regex patterns with fuzzing
  - ⚠️ Use regex linters (recheck, regexp-opt)

---

### 6. Elevation of Privilege (Authorization)

**Definition:** An attacker gains higher privileges than intended.

#### T-PRIV-001: Horizontal Privilege Escalation (Access Other Users' Data)
- **Threat:** Attacker accesses/modifies another user's resources (scenes, events, posts)
- **Attack Vector:** Direct object reference manipulation (change scene ID in request)
- **Impact:** HIGH - Unauthorized data access, data manipulation, privacy violation
- **Affected Assets:** User data, scenes, events, posts, memberships
- **Existing Mitigations:**
  - ✅ Authorization checks in handlers (e.g., `owner_did` comparison)
  - ✅ Membership verification for member-only scenes
  - ⚠️ **NEEDS VERIFICATION:** Consistent authorization across all endpoints
- **Risk Score:** **MEDIUM** (depends on endpoint coverage)
- **Residual Risk:** Missing authorization checks in some endpoints
- **Recommendations:**
  - 🔴 **HIGH PRIORITY:** Implement centralized authorization middleware
  - ⚠️ Create authorization matrix: User role × Resource × Action
  - ⚠️ Audit all CRUD endpoints for authorization checks
  - ⚠️ Automated tests: Attempt cross-user access for every endpoint
  - ⚠️ Use attribute-based access control (ABAC) framework

#### T-PRIV-002: Vertical Privilege Escalation (User to Admin)
- **Threat:** Regular user gains admin privileges
- **Attack Vector:** JWT manipulation, role field tampering, missing role checks
- **Impact:** CRITICAL - Full system compromise, data manipulation, user impersonation
- **Affected Assets:** Admin panel, system configuration, all user data
- **Existing Mitigations:**
  - ✅ User role in JWT claims (observed in `web/src/stores/authStore.ts`)
  - ⚠️ **NEEDS VERIFICATION:** Admin-only endpoint protection
  - ⚠️ **NEEDS VERIFICATION:** Role stored in database vs. JWT-only
- **Risk Score:** **MEDIUM** (role system exists but enforcement unclear)
- **Residual Risk:** Role checks missing from admin endpoints
- **Recommendations:**
  - 🔴 **HIGH PRIORITY:** Implement `RequireAdmin` middleware:
    ```go
    func RequireAdmin(next http.Handler) http.Handler {
      return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        claims := auth.GetClaims(r.Context())
        if claims.Role != "admin" {
          http.Error(w, "Forbidden", http.StatusForbidden)
          return
        }
        next.ServeHTTP(w, r)
      })
    }
    ```
  - ⚠️ Store user roles in database (source of truth)
  - ⚠️ Refresh JWT when user role changes
  - ⚠️ Audit trail for all admin actions
  - ⚠️ Two-factor authentication for admin accounts

#### T-PRIV-003: Scene Owner Privilege Bypass
- **Threat:** Non-owner gains scene management privileges (edit, delete, membership management)
- **Attack Vector:** Missing owner check, membership role confusion
- **Impact:** HIGH - Scene takeover, unauthorized content, membership manipulation
- **Affected Assets:** Scene ownership, scene settings, memberships
- **Existing Mitigations:**
  - ✅ `owner_did` field in scenes table
  - ⚠️ **NEEDS VERIFICATION:** Owner check enforcement in all scene management endpoints
- **Risk Score:** **MEDIUM** (owner model exists but enforcement needs verification)
- **Residual Risk:** Inconsistent owner checks
- **Recommendations:**
  - ⚠️ Implement `RequireSceneOwner` authorization check
  - ⚠️ Separate scene "admin" role (delegated management) from owner
  - ⚠️ Audit trail for ownership transfers
  - ⚠️ Require 2FA for scene deletion
  - ⚠️ Implement ownership transfer workflow with acceptance

#### T-PRIV-004: Stripe Connect Account Takeover
- **Threat:** Attacker links their Stripe account to another user's scene
- **Attack Vector:** CSRF on Stripe onboarding flow, state parameter manipulation
- **Impact:** CRITICAL - Financial fraud, payment redirection, revenue theft
- **Affected Assets:** Scene payouts, financial transactions, user trust
- **Existing Mitigations:**
  - ⚠️ **NEEDS VERIFICATION:** State parameter validation in Stripe webhook
  - ⚠️ **NEEDS VERIFICATION:** Scene owner verification before account linking
- **Risk Score:** **HIGH** (payment fraud potential)
- **Residual Risk:** Missing state validation, CSRF vulnerability
- **Recommendations:**
  - 🔴 **HIGH PRIORITY:** Implement state parameter validation:
    ```go
    // Generate: state = HMAC(scene_id + user_did + nonce, secret)
    // Verify: Validate HMAC before linking Stripe account
    ```
  - 🔴 Require scene owner authentication before onboarding
  - ⚠️ Log all Stripe account link events in audit trail
  - ⚠️ Email notification to scene owner when Stripe account linked
  - ⚠️ Implement 48-hour cooling-off period before first payout

#### T-PRIV-005: LiveKit Room Privilege Escalation
- **Threat:** Attacker gains host/organizer privileges in audio room
- **Attack Vector:** Token manipulation, role claim forgery, missing permissions check
- **Impact:** MEDIUM - Stream disruption, participant removal, room control
- **Affected Assets:** Live audio sessions, user experience
- **Existing Mitigations:**
  - ✅ LiveKit token generation server-side
  - ⚠️ **NEEDS VERIFICATION:** Room permissions based on scene membership
- **Risk Score:** **MEDIUM** (depends on permission enforcement)
- **Residual Risk:** Missing permission checks in room management API
- **Recommendations:**
  - ⚠️ Verify room permissions before token generation
  - ⚠️ Limit room control actions to scene owner + designated organizers
  - ⚠️ Implement "mute participant" audit logging
  - ⚠️ Client-side UI should reflect actual permissions (no fake buttons)
  - ⚠️ Rate limit room creation per user

#### T-PRIV-006: Alliance Weight Manipulation
- **Threat:** Attacker manipulates trust graph weights to boost ranking
- **Attack Vector:** Fake alliances, role multiplier exploitation, repeated alliance creation
- **Impact:** MEDIUM - Search ranking manipulation, trust system abuse
- **Affected Assets:** Trust graph integrity, search ranking fairness
- **Existing Mitigations:**
  - ✅ Role-based weight multipliers (organizer: 1.5x, artist: 1.3x, etc.)
  - ✅ Trust ranking feature flag (safe rollout)
  - ⚠️ **NEEDS VERIFICATION:** Alliance approval process
- **Risk Score:** **MEDIUM** (trust system manipulation risk)
- **Residual Risk:** Sybil attacks, fake alliances
- **Recommendations:**
  - ⚠️ Implement alliance approval workflow (recipient must accept)
  - ⚠️ Rate limit alliance creation (e.g., 10 per day per user)
  - ⚠️ Decay trust weights over time (require periodic confirmation)
  - ⚠️ Detect suspicious alliance patterns (e.g., reciprocal loops)
  - ⚠️ Manual review of high-weight alliances

---

## Risk Scoring Matrix

### Risk Level Definitions

| Risk Level | Definition | Likelihood | Impact | Action Required |
|------------|-----------|-----------|--------|-----------------|
| **CRITICAL** | Immediate threat to core security or privacy | High | Severe | Fix within 7 days |
| **HIGH** | Significant security weakness | Medium-High | Major | Fix within 30 days |
| **MEDIUM** | Moderate security concern | Medium | Moderate | Fix within 90 days |
| **LOW** | Minor security issue or well-mitigated | Low | Minor | Fix in next release |

### Threat Summary by Risk Level

**CRITICAL (0 immediate threats):**
- ✅ No critical unmitigated threats identified
- ⚠️ Several potential critical threats if mitigations fail (T-SPOOF-001, T-TAMP-001, T-INFO-001)

**HIGH (7 threats requiring immediate attention):**
- T-SPOOF-002: JWT Secret Exposure (no rotation mechanism)
- T-SPOOF-004: DID Spoofing (verification unclear)
- T-INFO-003: EXIF Metadata Leakage (integration gap)
- T-DOS-002: Resource Exhaustion via Large Uploads (no limits)
- T-PRIV-001: Horizontal Privilege Escalation (needs audit)
- T-PRIV-002: Vertical Privilege Escalation (admin middleware needed)
- T-PRIV-004: Stripe Connect Account Takeover (state validation needed)

**MEDIUM (16 threats):**
- T-SPOOF-003, T-SPOOF-005, T-TAMP-002, T-TAMP-005, T-TAMP-006
- T-INFO-002, T-INFO-004, T-INFO-006, T-INFO-007
- T-DOS-001, T-DOS-003, T-DOS-004
- T-PRIV-003, T-PRIV-005, T-PRIV-006

**LOW (15 threats - well-mitigated):**
- T-SPOOF-001, T-TAMP-001, T-TAMP-003, T-TAMP-004
- T-REPUD-001, T-REPUD-002, T-REPUD-003
- T-INFO-001, T-INFO-005
- T-DOS-005, T-DOS-006

---

## Mitigation Mapping

### Implemented Mitigations (✅ Complete)

| Mitigation | Threats Addressed | Implementation Status | Location |
|-----------|-------------------|----------------------|----------|
| JWT Authentication (HS256) | T-SPOOF-001 | ✅ Complete | `internal/auth/jwt.go` |
| Location Consent Enforcement | T-TAMP-004, T-INFO-001 | ✅ Complete | Models + repositories + DB constraints |
| EXIF Metadata Stripping | T-INFO-003 | ✅ Complete | `internal/image/processor.go` |
| Audit Logging | T-REPUD-001 | ✅ Complete | `internal/audit/` |
| Rate Limiting | T-DOS-001 | ✅ Complete | `internal/middleware/ratelimit.go` |
| Scene Visibility Controls | T-INFO-005 | ✅ Complete | Scene model + handlers |
| Structured Logging | T-REPUD-002 | ✅ Complete | `internal/middleware/logging.go` |
| Input Validation | T-TAMP-003 | ✅ Partial | API handlers |
| Database Constraints | T-TAMP-004 | ✅ Complete | Migration `000000_initial_schema.up.sql` |

### Partially Implemented Mitigations (⚠️ Needs Work)

| Mitigation | Threats Addressed | Gaps | Priority |
|-----------|-------------------|------|----------|
| CSRF Protection | T-SPOOF-005 | No explicit CSRF tokens | Medium |
| Content Security Policy | T-TAMP-002 | No CSP headers | **HIGH** |
| File Upload Limits | T-DOS-002 | No size/quota limits visible | **HIGH** |
| Authorization Checks | T-PRIV-001, T-PRIV-003 | Consistency not verified | **HIGH** |
| Error Sanitization | T-INFO-004 | Needs verification in prod | Medium |
| DID Verification | T-SPOOF-004 | Process not documented | **HIGH** |

### Missing Mitigations (🔴 Not Implemented)

| Mitigation | Threats Addressed | Recommendation | Priority |
|-----------|-------------------|----------------|----------|
| JWT Secret Rotation | T-SPOOF-002 | Dual-key rotation system | **HIGH** |
| Token Revocation | T-SPOOF-003 | Redis-backed revocation list | Medium |
| Audit Log Hash Chain | T-TAMP-005 | Tamper detection | Medium |
| Admin Authorization | T-PRIV-002 | RequireAdmin middleware | **HIGH** |
| Stripe State Validation | T-PRIV-004 | HMAC-based state parameter | **HIGH** |
| Upload Quotas | T-DOS-002 | Per-user storage limits | **HIGH** |
| HTTP Timeouts | T-DOS-003 | Caddy + Go server config | Medium |
| Connection Pool Limits | T-DOS-004 | Database pool configuration | Medium |

---

## Implementation Roadmap

### Phase 1: Critical Security Gaps (0-30 days)

**Priority 1: Authentication & Authorization**
- [ ] **T-SPOOF-002:** Implement JWT dual-key rotation system
  - Add `JWT_SECRET_PRIMARY` and `JWT_SECRET_SECONDARY` config
  - Modify `ValidateToken()` to try both keys
  - Create rotation procedure document
  - **Estimated Effort:** 3 days
- [ ] **T-PRIV-002:** Implement `RequireAdmin` middleware
  - Add admin role enforcement to admin endpoints
  - Store roles in database (source of truth)
  - Add automated tests for admin access control
  - **Estimated Effort:** 2 days
- [ ] **T-PRIV-001:** Audit authorization checks across all endpoints
  - Create authorization matrix (user × resource × action)
  - Add missing owner/membership checks
  - Implement automated cross-user access tests
  - **Estimated Effort:** 5 days

**Priority 2: Web Application Security**
- [ ] **T-TAMP-002:** Implement Content Security Policy
  - Add CSP middleware to API server
  - Configure strict CSP headers
  - Test with browser console, adjust for violations
  - **Estimated Effort:** 2 days
- [ ] **T-SPOOF-004:** Document and verify DID authentication
  - Document current DID verification process
  - Implement DID document signature verification
  - Add DID change audit logging
  - **Estimated Effort:** 3 days
- [ ] **T-PRIV-004:** Implement Stripe state validation
  - Generate HMAC-based state parameter
  - Verify state before linking Stripe accounts
  - Add audit logging for account links
  - **Estimated Effort:** 2 days

**Priority 3: Resource Protection**
- [ ] **T-DOS-002:** Implement file upload limits
  - Add max file size validation (10MB images, 50MB audio)
  - Implement per-user storage quota (1GB default)
  - Add request body size limit middleware
  - **Estimated Effort:** 3 days
- [ ] **T-INFO-003:** Integrate EXIF stripping with upload API
  - Call `image.Process()` in upload handler
  - Add automated EXIF verification tests
  - Apply to all image upload endpoints
  - **Estimated Effort:** 2 days

### Phase 2: Security Hardening (30-90 days)

**Identity & Access**
- [ ] **T-SPOOF-003:** Implement token revocation
  - Redis-backed revocation list
  - Add revoke endpoint
  - Token fingerprinting (user-agent + IP)
  - **Estimated Effort:** 3 days
- [ ] **T-SPOOF-005:** Implement CSRF protection
  - Add CSRF token generation/validation
  - Enforce SameSite cookies
  - Validate Referer/Origin headers
  - **Estimated Effort:** 2 days

**Data Protection**
- [ ] **T-TAMP-005:** Implement audit log hash chain
  - Add `previous_log_hash` column
  - Compute hash on write
  - Create integrity verification script
  - **Estimated Effort:** 3 days
- [ ] **T-INFO-002:** Pseudonymize PII in logs
  - Hash DIDs with daily rotating salt
  - Truncate IP addresses (/24 for IPv4)
  - Document in privacy policy
  - **Estimated Effort:** 2 days

**DoS Prevention**
- [ ] **T-DOS-003:** Configure HTTP timeouts
  - Add Caddy timeout configuration
  - Set Go http.Server timeouts
  - Test with slow client simulations
  - **Estimated Effort:** 1 day
- [ ] **T-DOS-004:** Configure database connection pool
  - Set max open/idle connections
  - Implement query timeouts
  - Monitor connection usage
  - **Estimated Effort:** 1 day

### Phase 3: Advanced Security (90-180 days)

**Detection & Response**
- [ ] Implement security monitoring dashboards
  - JWT validation failures
  - Rate limit violations
  - Authorization failures
  - Anomalous upload patterns
  - **Estimated Effort:** 5 days
- [ ] Set up automated alerts
  - High error rates (>5% of requests)
  - Repeated 401/403/429 responses
  - Failed DID verifications
  - Suspicious alliance patterns
  - **Estimated Effort:** 3 days

**Resilience & Recovery**
- [ ] Implement secret rotation automation
  - Scheduled rotation every 90 days
  - Automated key distribution
  - Zero-downtime rotation process
  - **Estimated Effort:** 5 days
- [ ] Create incident response playbook
  - JWT secret compromise procedure
  - Database credential rotation procedure
  - Third-party API key rotation procedure
  - **Estimated Effort:** 3 days

**Compliance & Governance**
- [ ] Implement data retention automation
  - Access logs: 90 days
  - Error logs: 30 days
  - Audit logs: 7 years
  - **Estimated Effort:** 3 days
- [ ] Create security review checklist
  - New endpoint authorization review
  - Privacy impact assessment for location features
  - Third-party integration security review
  - **Estimated Effort:** 2 days

---

## Testing & Validation

### Security Test Suite

**Authentication & Authorization**
- [ ] Automated test: JWT algorithm confusion attack attempt
- [ ] Automated test: Expired token rejection
- [ ] Automated test: Cross-user data access attempts (all CRUD endpoints)
- [ ] Automated test: Admin endpoint access by non-admin users
- [ ] Automated test: Scene owner privilege verification

**Input Validation & Tampering**
- [ ] SQL injection testing (automated with sqlmap or custom)
- [ ] XSS testing (automated with ZAP or custom)
- [ ] CSRF testing (manual verification)
- [ ] File upload malware testing (polyglot files, wrong MIME types)
- [ ] Request body size limit testing (oversized payloads)

**Privacy & Information Disclosure**
- [ ] Location consent enforcement tests (existing: `internal/scene/privacy_test.go`)
- [ ] EXIF metadata removal verification (all image uploads)
- [ ] Error message sanitization (verbose errors in dev, generic in prod)
- [ ] Scene visibility enforcement (public/members-only/hidden)

**DoS & Resource Limits**
- [ ] Rate limiting effectiveness (botnet simulation)
- [ ] Large file upload testing (storage exhaustion)
- [ ] Slowloris attack simulation
- [ ] Database connection pool exhaustion testing

**Audit & Logging**
- [ ] Audit log coverage verification (all sensitive endpoints)
- [ ] Audit log integrity testing (hash chain verification)
- [ ] Log PII detection (automated scan for exposed DIDs, IPs)

### Red Team Exercises

**Annual Red Team Scenarios:**

1. **Location Privacy Bypass Attempt**
   - Goal: Extract precise coordinates without consent
   - Methods: API manipulation, SQL injection, log access
   - Success Criteria: Zero successful extractions

2. **Financial Fraud Simulation**
   - Goal: Redirect scene payouts to attacker's Stripe account
   - Methods: CSRF, state manipulation, session hijacking
   - Success Criteria: All attempts detected and blocked

3. **Privilege Escalation Campaign**
   - Goal: Gain admin access from regular user account
   - Methods: JWT manipulation, authorization bypass, role field tampering
   - Success Criteria: No successful escalations

4. **Data Exfiltration Exercise**
   - Goal: Extract all user DIDs and locations
   - Methods: Database compromise, log aggregation, API scraping
   - Success Criteria: All attempts detected within 1 hour

---

## Stakeholder Review

### Review Process

**Security Team Review (Completed):** ✅
- Threat identification: 38 threats across 6 STRIDE categories
- Risk scoring: 0 critical (immediate), 7 high, 16 medium, 15 low
- Mitigation mapping: 9 complete, 6 partial, 8 missing
- Implementation roadmap: 3 phases over 180 days

**Engineering Team Review (Pending):**
- Review technical feasibility of recommendations
- Estimate implementation effort for each phase
- Identify additional constraints or dependencies
- Prioritize based on development roadmap

**Product Team Review (Pending):**
- Review impact on user experience (e.g., CAPTCHA, 2FA)
- Prioritize features vs. security hardening
- Align roadmap with product goals
- User communication strategy for security changes

**Compliance Team Review (Pending):**
- Verify GDPR compliance measures
- Review data retention policies
- Validate audit logging coverage
- Regulatory reporting requirements

**Executive Review (Pending):**
- Approve security investment (time, cost, resources)
- Sign off on acceptable residual risks
- Set security budget and timeline
- Strategic security objectives

### Sign-Off

| Stakeholder | Role | Review Date | Approval |
|-------------|------|-------------|----------|
| Security Team | Threat Model Author | 2026-02-19 | ✅ Approved |
| Engineering Lead | Implementation Owner | _Pending_ | ⏳ |
| Product Manager | Feature Prioritization | _Pending_ | ⏳ |
| Compliance Officer | Regulatory Compliance | _Pending_ | ⏳ |
| CTO/VP Engineering | Executive Sponsor | _Pending_ | ⏳ |

---

## Appendices

### A. STRIDE Methodology Reference

**STRIDE** is a threat modeling framework developed by Microsoft for identifying security threats:

- **S**poofing: Illegitimate use of authentication credentials
- **T**ampering: Malicious modification of data
- **R**epudiation: Denying actions without proof to the contrary
- **I**nformation Disclosure: Exposure of confidential information
- **D**enial of Service: Making a system unavailable
- **E**levation of Privilege: Gaining unauthorized access rights

### B. Security Resources

**Internal Documentation:**
- [Privacy Technical Overview](./PRIVACY.md)
- [Rate Limiting Guide](./RATE_LIMITING.md)
- [Audit Logging Documentation](../internal/audit/README.md)
- [Configuration Reference](./CONFIGURATION.md)
- [Architecture Overview](./ARCHITECTURE.md)

**External References:**
- OWASP Top 10: https://owasp.org/www-project-top-ten/
- OWASP ASVS: https://owasp.org/www-project-application-security-verification-standard/
- CWE Top 25: https://cwe.mitre.org/top25/
- NIST Cybersecurity Framework: https://www.nist.gov/cyberframework
- STRIDE Documentation: https://learn.microsoft.com/en-us/azure/security/develop/threat-modeling-tool-threats

### C. Glossary

| Term | Definition |
|------|------------|
| **DID** | Decentralized Identifier - User identity in AT Protocol |
| **JWT** | JSON Web Token - Bearer token for authentication |
| **EXIF** | Exchangeable Image File Format - Metadata in images |
| **CSP** | Content Security Policy - HTTP header to prevent XSS |
| **CORS** | Cross-Origin Resource Sharing - HTTP access control |
| **CSRF** | Cross-Site Request Forgery - Unauthorized action on behalf of user |
| **Geohash** | Geographic coordinate encoding system |
| **R2** | Cloudflare R2 - S3-compatible object storage |
| **Jetstream** | AT Protocol real-time data firehose |
| **STRIDE** | Threat modeling framework (Spoofing, Tampering, Repudiation, Information Disclosure, Denial of Service, Elevation of Privilege) |
| **ReDoS** | Regular Expression Denial of Service |
| **MITM** | Man-in-the-Middle attack |
| **PII** | Personally Identifiable Information |

### D. Revision History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | 2026-02-19 | Security Team | Initial STRIDE analysis with 38 threats, risk scoring, and implementation roadmap |

---

**Document Classification:** Internal Use  
**Next Review Date:** 2026-08-19 (6 months)  
**Document Owner:** Security Team  
**Related Issues:** #308, #130, #20

