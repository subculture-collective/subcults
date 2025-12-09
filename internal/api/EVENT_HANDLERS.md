# Event Handlers Documentation

This document describes the HTTP handlers for event CRUD operations in the Subcults API.

## Overview

Event handlers provide endpoints for creating, updating, and retrieving events within scenes. Events drive temporal discovery on the platform, enabling users to schedule and discover time-based activities in music scenes.

## Handlers

### EventHandlers

The `EventHandlers` struct manages all event-related HTTP endpoints.

```go
type EventHandlers struct {
    eventRepo scene.EventRepository
    sceneRepo scene.SceneRepository
}
```

**Constructor:**

```go
func NewEventHandlers(eventRepo scene.EventRepository, sceneRepo scene.SceneRepository) *EventHandlers
```

## Endpoints

### POST /events - Create Event

Creates a new event within a scene.

**Request Body:**

```json
{
  "scene_id": "uuid",
  "title": "Event Title",
  "description": "Optional description",
  "allow_precise": true,
  "precise_point": {
    "lat": 40.7128,
    "lng": -74.0060
  },
  "coarse_geohash": "dr5regw",
  "tags": ["tag1", "tag2"],
  "starts_at": "2024-12-25T20:00:00Z",
  "ends_at": "2024-12-25T23:00:00Z"
}
```

**Required Fields:**
- `scene_id`: UUID of the parent scene
- `title`: Event title (3-80 characters)
- `coarse_geohash`: Geohash for approximate location
- `starts_at`: Event start time (RFC3339 format)

**Optional Fields:**
- `description`: Event description
- `allow_precise`: Privacy consent for precise location (default: false)
- `precise_point`: Precise GPS coordinates (only stored if `allow_precise` is true)
- `tags`: Array of categorization tags
- `ends_at`: Event end time (must be after `starts_at`)

**Authorization:**
- Requires authentication (JWT token)
- User must be the owner of the parent scene

**Validations:**
- Title length: 3-80 characters
- `coarse_geohash` is required and non-empty
- If `ends_at` is provided, `starts_at` must be before `ends_at`
- `scene_id` must reference an existing, non-deleted scene
- HTML sanitization applied to `title`, `description`, and `tags`

**Privacy Enforcement:**
- If `allow_precise` is false, `precise_point` is cleared before storage
- Repository automatically enforces location consent

**Success Response (201 Created):**

```json
{
  "id": "event-uuid",
  "scene_id": "scene-uuid",
  "title": "Event Title",
  "description": "Description",
  "allow_precise": true,
  "precise_point": {
    "lat": 40.7128,
    "lng": -74.0060
  },
  "coarse_geohash": "dr5regw",
  "tags": ["tag1", "tag2"],
  "status": "scheduled",
  "starts_at": "2024-12-25T20:00:00Z",
  "ends_at": "2024-12-25T23:00:00Z",
  "created_at": "2024-12-09T18:00:00Z",
  "updated_at": "2024-12-09T18:00:00Z"
}
```

**Error Responses:**

| Status | Error Code | Description |
|--------|------------|-------------|
| 400 | `bad_request` | Invalid JSON in request body |
| 400 | `validation_error` | Title length invalid, or missing required field |
| 400 | `invalid_time_range` | Start time is not before end time |
| 401 | `auth_failed` | Authentication required |
| 403 | `forbidden` | User does not own the parent scene |
| 404 | `not_found` | Parent scene not found or deleted |
| 500 | `internal_error` | Server error during creation |

### PATCH /events/{id} - Update Event

Updates an existing event.

**URL Parameters:**
- `id`: Event UUID

**Request Body (all fields optional):**

```json
{
  "title": "Updated Title",
  "description": "Updated description",
  "tags": ["new", "tags"],
  "allow_precise": false,
  "coarse_geohash": "dr5regx",
  "starts_at": "2024-12-26T20:00:00Z",
  "ends_at": "2024-12-26T23:00:00Z"
}
```

**Immutable Fields:**
- `scene_id`: Cannot be changed after creation

**Update Restrictions:**
- `starts_at` can only be updated if the event is still in the future
- Time window validation applies: `starts_at` must be before `ends_at`

**Authorization:**
- Requires authentication (JWT token)
- User must be the owner of the parent scene

**Validations:**
- If `title` is provided, must be 3-80 characters
- If `coarse_geohash` is provided, must be non-empty
- Time window validation: `starts_at` < `ends_at`
- Cannot update `starts_at` for past events
- HTML sanitization applied to updated fields

**Success Response (200 OK):**

```json
{
  "id": "event-uuid",
  "scene_id": "scene-uuid",
  "title": "Updated Title",
  "description": "Updated description",
  "allow_precise": false,
  "coarse_geohash": "dr5regx",
  "tags": ["new", "tags"],
  "status": "scheduled",
  "starts_at": "2024-12-26T20:00:00Z",
  "ends_at": "2024-12-26T23:00:00Z",
  "created_at": "2024-12-09T18:00:00Z",
  "updated_at": "2024-12-09T18:30:00Z"
}
```

**Error Responses:**

| Status | Error Code | Description |
|--------|------------|-------------|
| 400 | `bad_request` | Invalid JSON or missing event ID |
| 400 | `validation_error` | Validation failed or cannot update past event |
| 400 | `invalid_time_range` | Start time is not before end time |
| 401 | `auth_failed` | Authentication required |
| 403 | `forbidden` | User does not own the parent scene |
| 404 | `not_found` | Event or parent scene not found |
| 500 | `internal_error` | Server error during update |

### GET /events/{id} - Get Event

Retrieves a single event by ID.

**URL Parameters:**
- `id`: Event UUID

**Authorization:**
- Public endpoint (no authentication required)

**Privacy Enforcement:**
- If `allow_precise` is false, `precise_point` is excluded from response
- Repository automatically enforces location consent

**Success Response (200 OK):**

```json
{
  "id": "event-uuid",
  "scene_id": "scene-uuid",
  "title": "Event Title",
  "description": "Description",
  "allow_precise": true,
  "precise_point": {
    "lat": 40.7128,
    "lng": -74.0060
  },
  "coarse_geohash": "dr5regw",
  "tags": ["tag1", "tag2"],
  "status": "scheduled",
  "starts_at": "2024-12-25T20:00:00Z",
  "ends_at": "2024-12-25T23:00:00Z",
  "created_at": "2024-12-09T18:00:00Z",
  "updated_at": "2024-12-09T18:00:00Z"
}
```

**Error Responses:**

| Status | Error Code | Description |
|--------|------------|-------------|
| 400 | `bad_request` | Missing or invalid event ID |
| 404 | `not_found` | Event not found |
| 500 | `internal_error` | Server error during retrieval |

## Validation Rules

### Title Validation

```go
// Title length constraints
const (
    MinEventTitleLength = 3
    MaxEventTitleLength = 80
)
```

- Minimum length: 3 characters
- Maximum length: 80 characters
- Whitespace is trimmed before validation
- HTML sanitization applied after validation

### Time Window Validation

```go
func validateTimeWindow(startsAt time.Time, endsAt *time.Time) string
```

- If `ends_at` is provided, `starts_at` must be strictly before `ends_at`
- Equal times are rejected (error code: `invalid_time_range`)
- Past events cannot have `starts_at` updated

### Coarse Geohash Validation

- Required on creation
- Must be non-empty string
- Used for approximate location-based discovery

## Security Considerations

### HTML Sanitization

All user-provided text fields are sanitized using `html.EscapeString()`:
- `title`
- `description`
- `tags` (each element individually)

### Authorization

- Event creation and updates require scene ownership verification
- Uses `isSceneOwner()` helper to check authorization
- Uniform error messages prevent user enumeration

### Privacy Enforcement

- Location consent enforced at multiple layers:
  1. Repository level (automatic)
  2. Handler level (explicit check in GET handler)
- `precise_point` automatically cleared if `allow_precise` is false
- Prevents precise location exposure without explicit consent

## Testing

Comprehensive test coverage includes:

- **Success Cases:**
  - Event creation with all fields
  - Event updates (title, description, tags, times)
  - Event retrieval

- **Validation Tests:**
  - Title too short (< 3 chars)
  - Title too long (> 80 chars)
  - Missing coarse_geohash
  - Invalid time window (end before start, equal times)
  - Cannot update past event start time

- **Authorization Tests:**
  - Unauthorized creation (non-scene-owner)
  - Unauthorized update (non-scene-owner)

- **Privacy Tests:**
  - Privacy enforcement on creation (allow_precise=false)
  - Privacy enforcement on retrieval (precise_point hidden)

Run tests:

```bash
go test -v ./internal/api/event_handlers_test.go
```

## Error Codes

| Code | Usage |
|------|-------|
| `invalid_time_range` | Start time is not before end time |
| `validation_error` | Generic input validation failure |
| `auth_failed` | Authentication required |
| `forbidden` | User lacks permission (not scene owner) |
| `not_found` | Event or scene not found |
| `bad_request` | Malformed request (invalid JSON, missing ID) |
| `internal_error` | Server error |

## Future Enhancements

- Event search and filtering endpoints
- Event listing by scene
- Event status transitions (scheduled → live → ended)
- Event cancellation endpoint
- Recurring events support
- Event attendance/RSVP functionality
- Integration with LiveKit for live streaming events
