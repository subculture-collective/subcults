# Issue Accountability Report

## Canonical execution order (from #416)

## Epic #3 — Epic: Backend Core (Go API Service)
- [EPIC] #3 Epic: Backend Core (Go API Service) (open)
- #25 Task: JWT Auth Module (closed)
- #29 Task: Structured Logging Middleware (closed)
- #34 Task: Rate Limiting Middleware (closed)
- #37 Task: Graceful Shutdown Handling (closed)
- #46 Task: API Config Loader (koanf) (closed)
- #52 Task: Standard Error Response Format (closed)
- #53 Task: Request ID Middleware (closed)

## Epic #4 — Epic: Database & Migrations (Neon + PostGIS)
- [EPIC] #4 Epic: Database & Migrations (Neon + PostGIS) (open)
- #30 Task: Migration Tool Setup (golang-migrate) (closed)
- #35 Task: PostGIS Extension Migration (closed)
- #36 Task: Posts Table Migration (closed)
- #41 Task: Users Table Migration (closed)
- #42 Task: Events Table Migration (closed)
- #44 Task: Memberships & Alliances Tables (closed)
- #51 Task: Scenes Table Migration (closed)

## Epic #6 — Epic: Privacy & Safety
- [EPIC] #6 Epic: Privacy & Safety (open)
- #27 Task: Precise Coordinate Consent Enforcement (closed)
- #32 Task: Geohash Rounding Utility (closed)
- #45 Task: Privacy Policy Documentation Draft (closed)
- #47 Task: Access Audit Logging Integration (closed)
- #50 Task: Privacy Test Suite (closed)
- #54 Task: Image EXIF Stripping Service (closed)

## Epic #305 — Epic: Jetstream Indexer - Complete Real-Time Data Ingestion
- [EPIC] #305 Epic: Jetstream Indexer - Complete Real-Time Data Ingestion (open)
- #338 CBOR record parsing for AT Protocol records (closed)
- #339 Backpressure handling in Jetstream consumer (closed)
- #340 Comprehensive Jetstream indexer testing (closed)
- #341 Transaction consistency and atomicity in indexing (open)
- #342 Indexer metrics and monitoring (closed)
- #343 Entity mapping from AT Protocol to domain models (open)
- #344 Reconnection and resume logic for Jetstream (open)
- #436 Cleanup: Resolve AT Protocol schema dependency mismatch in #305 (open)

## Epic #298 — Epic: Telemetry and Error Logging Infrastructure
- [EPIC] #298 Epic: Telemetry and Error Logging Infrastructure (open)
- #299 Implement /telemetry endpoint for event batching (open)
- #300 Implement /api/log/client-error endpoint for error collection (open)
- #301 Create database schema for telemetry and error logs (open)
- #302 Implement API handlers for telemetry and error logging (open)

## Epic #307 — Epic: Observability, Monitoring & Operations
- [EPIC] #307 Epic: Observability, Monitoring & Operations (open)
- #360 Slow query logging and performance monitoring (open)
- #361 Health and readiness endpoints (closed)
- #362 OpenTelemetry tracing setup (closed)
- #363 Streaming service metrics and monitoring (open)
- #364 HTTP request metrics middleware (closed)
- #365 Structured logging standardization (closed)
- #366 Grafana dashboards and SLO definitions (open)
- #367 Background job metrics and progress tracking (closed)

## Epic #10 — Epic: Scene Management
- [EPIC] #10 Epic: Scene Management (open)
- #74 Task: Scene Create / Update / Delete Handlers (closed)
- #75 Task: Scene Membership Request Workflow (closed)
- #76 Task: Scene Palette & Theme Update Endpoint (closed)
- #77 Task: Scene Visibility & Privacy Enforcement (closed)
- #78 Task: Soft-Delete Filtering & Exclusion Tests (closed)
- #79 Task: Scene Owner Dashboard & Listing Endpoint (closed)
- #80 Task: Scene Name Sanitization & Uniqueness Validation (closed)

## Epic #15 — Epic: Event System
- [EPIC] #15 Epic: Event System (open)
- #81 Task: Event Create / Update / Fetch Handlers (closed)
- #82 Task: Event Cancellation Endpoint (closed)
- #83 Task: RSVP Table & Endpoints (closed)
- #84 Task: Bbox + Time Range Event Search Endpoint (closed)
- #85 Task: Stream Linkage in Event Payload (closed)
- #86 Task: Event Validation Test Matrix (closed)
- #200 Task: Tailwind CSS Integration & Design System Foundation (closed)

## Epic #17 — Epic: Post & Content
- [EPIC] #17 Epic: Post & Content (open)
- #87 Task: Post Create / Update / Delete Handlers (closed)
- #88 Task: R2 Signed URL Generation Service (closed)
- #89 Task: Feed Aggregation (Scene/Event) Endpoints (closed)
- #90 Task: Moderation Label Application & Filtering (closed)
- #91 Task: Attachment Metadata Extraction & EXIF Sanitation Link (closed)
- #92 Task: Post Feed Pagination & Cursor Integrity Tests (closed)
- #166 Task: Performance Guide & Budgets Documentation (open)
- #167 Task: Automated Query EXPLAIN Capture & Regression Diff (open)
- #168 Task: Cache-Control Headers & CDN Strategy Implementation (open)
- #169 Task: Streaming Latency Improvement Pipeline (open)
- #170 Task: Map Load & Rendering Profiling Tests (open)
- #171 Task: Ranking Calibration Tool & Simulation Harness (open)
- #172 Task: Trust Recompute Job Tuning & Scheduling (open)
- #173 Task: Feed Aggregation Query Optimization & Caching Layer (open)

## Epic #24 — Epic: Trust Graph & Alliances
- [EPIC] #24 Epic: Trust Graph & Alliances (open)
- #93 Task: Alliance Create / Update / Delete Endpoints (closed)
- #94 Task: Trust Score API Endpoint (closed)
- #95 Task: Trust Weight Validation & Role Multipliers (closed)
- #96 Task: Trust Ranking Integration Feature Flag (closed)
- #100 Task: Trust Recompute Performance Metrics & Monitoring (closed)

## Epic #22 — Epic: Payments & Revenue (Stripe Connect)
- [EPIC] #22 Epic: Payments & Revenue (Stripe Connect) (open)
- #68 Task: Stripe Connect Onboarding Link Endpoint (closed)
- #69 Task: Checkout Session Creation with Platform Fee (closed)
- #70 Task: Payment Record Model & Migration (closed)
- #71 Task: Stripe Webhook Handler (Signature Verification) (closed)
- #72 Task: Payment Status Polling Endpoint (closed)
- #73 Task: Idempotency Key Strategy & Middleware (closed)

## Epic #14 — Epic: Mapping & Geo Frontend
- [EPIC] #14 Epic: Mapping & Geo Frontend (open)
- #56 Task: Map Component Integration (MapLibre + MapTiler) (closed)
- #57 Task: Scene & Event Clustering Logic (closed)
- #58 Task: Bbox Query Hook & Debounce (closed)
- #59 Task: Jitter Visualization Overlay (closed)
- #60 Task: Detail Panel & Marker Interaction (closed)
- #61 Task: Map Performance & Render Profiling (closed)

## Epic #303 — Epic: Frontend UX Shell & Advanced Features Completion
- [EPIC] #303 Epic: Frontend UX Shell & Advanced Features Completion (open)
- #312 Global layout wrapper with header/sidebar/main area (open)
- #313 Global search bar with dropdown results (open)
- #314 Performance budget monitoring and Lighthouse CI (open)
- #315 App routing completion (all pages) (open)
- #316 User authentication flow UI components (open)
- #317 Image optimization and responsive images (open)
- #318 Component unit tests extending coverage to 70%+ (open)
- #319 Complete i18n implementation and translations (open)
- #320 Code splitting and lazy loading routes (open)
- #321 Mobile-first responsive design (<480px) (open)
- #322 PWA manifest and service worker offline support (open)
- #323 WCAG 2.1 Level AA compliance audit (open)
- #324 Scene organizer settings and customization (open)
- #325 Dark mode implementation with persistence (open)
- #326 Integration tests for critical user flows (open)
- #327 Component styling consistency and design system (open)
- #328 User settings page implementation (open)

## Epic #304 — Epic: Complete Search & Discovery Implementation
- [EPIC] #304 Epic: Complete Search & Discovery Implementation (open)
- #329 Search performance optimization and indexing (open)
- #330 Scene full-text search implementation (open)
- #331 Unified search across all content types (open)
- #332 Event search with date range filtering (open)
- #333 Post content search implementation (open)
- #334 Search analytics and click tracking (open)
- #335 Search result autocomplete and suggestions (open)
- #336 Search pagination and cursor integrity (open)
- #337 Frontend search UI and results page (open)

## Epic #306 — Epic: LiveKit Streaming - Complete WebRTC Audio Implementation
- [EPIC] #306 Epic: LiveKit Streaming - Complete WebRTC Audio Implementation (open)
- #345 Audio quality adaptation and metrics (closed)
- #346 Stream analytics and engagement metrics (closed)
- #347 Stream E2E testing with mock LiveKit (closed)
- #348 Organizer stream controls (mute, kick) (closed)
- #349 Participant state synchronization in streams (closed)
- #350 Frontend stream join UI and controls (open)
- #351 LiveKit room creation and lifecycle handlers (closed)

## Epic #308 — Epic: Security Hardening & Compliance
- [EPIC] #308 Epic: Security Hardening & Compliance (open)
- #352 Comprehensive audit logging for compliance (open)
- #353 Rate limiting implementation (closed)
- #354 Security threat model (STRIDE analysis) (open)
- #355 Dependency vulnerability scanning (open)
- #356 JWT secret rotation and key management (open)
- #357 Input validation and sanitization layer (open)
- #358 CORS security configuration (open)
- #359 Security headers and CSP implementation (open)

## Epic #11 — Epic: Performance & Optimization
- [EPIC] #11 Epic: Performance & Optimization (open)

## Epic #385 — Epic: Comprehensive Testing & Quality Assurance
- [EPIC] #385 Epic: Comprehensive Testing & Quality Assurance (open)
- #388 Repository integration tests with test database (testcontainers) (open)
- #389 Unit test expansion for API handlers (scenes, events, posts) (open)
- #390 Test coverage reporting and CI gates (open)
- #391 Unit test expansion for core packages (auth, geo, ranking, validation, streams) (open)
- #392 k6 load testing for API endpoints and streaming (open)
- #393 Frontend component unit tests with React Testing Library (open)
- #394 Playwright E2E smoke tests for critical user flows (open)
- #395 Unit test expansion for repositories (Scene, Event, Stream, Payment) (open)

## Epic #386 — Epic: Deployment & Infrastructure Operations
- [EPIC] #386 Epic: Deployment & Infrastructure Operations (open)
- #396 Production Frontend Docker image with nginx (open)
- #397 Container registry setup and image scanning (open)
- #398 Database backup and disaster recovery procedures (open)
- #399 Blue-green deployment strategy (open)
- #400 Pre-deployment validation checklist (open)
- #401 Production API Docker image optimization (open)
- #402 Helm chart creation for multi-environment deployment (open)
- #403 Kubernetes deployment manifests (open)

## Epic #387 — Epic: Documentation & Developer Reference
- [EPIC] #387 Epic: Documentation & Developer Reference (open)
- #404 Frontend development guide (open)
- #405 Backend development guide (open)
- #406 Performance guide and optimization strategies (open)
- #407 OpenAPI specification for all API endpoints (open)
- #408 Testing guide (open)
- #409 Developer onboarding guide (open)
- #410 Troubleshooting guide for common issues (open)
- #411 System architecture documentation with diagrams (open)
- #412 Configuration and environment variables documentation (open)
- #413 Privacy policy and compliance documentation (open)
- #414 Operational runbooks for common incidents (open)
- #415 Architecture decision records (ADRs) (open)

## Epic #13 — Epic: Backfill & Migration
- [EPIC] #13 Epic: Backfill & Migration (open)

## Epic #7 — Epic: Future: OpenSearch Integration
- [EPIC] #7 Epic: Future: OpenSearch Integration (open)

## Epic #9 — Epic: Future: Mobile Support
- [EPIC] #9 Epic: Future: Mobile Support (open)

## Independent issues (not epics, not sub-issues)

- #108 Task: Theming System & Dark Mode Toggle (closed)
- #206 ## Pull request overview (closed)
- #435 Cleanup: Normalize epic dependencies to canonical roadmap (#416) (closed)

---

Notes:
- This report is derived from issue bodies that include an `Epic:` line.
- Any issues missing `Epic:` lines will appear under Independent issues.
- This document does not assign work; it only enumerates order and independence.
