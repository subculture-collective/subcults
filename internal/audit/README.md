# Audit Logging

The audit logging package provides comprehensive access tracking for sensitive endpoints and operations, supporting compliance requirements and incident response.

## Overview

Audit logs record access events with:
- User identity (DID)
- Entity type and ID accessed
- Action performed
- Timestamp (UTC)
- Request metadata (request ID, IP address without port, user agent)

## Privacy & Compliance Notice

**⚠️ Important: PII Storage**

Audit logs contain Personally Identifiable Information (PII):
- **User DIDs**: Decentralized identifiers linking to user accounts
- **IP Addresses**: Client IP addresses (without port numbers)
- **User Agents**: Browser/client information

### Compliance Considerations

1. **Data Retention**: Audit logs should be retained according to your organization's compliance requirements (GDPR, CCPA, etc.). Implement automatic retention policies to delete old logs.

2. **Access Controls**: Audit logs themselves must be protected from unauthorized access. Only authorized personnel (security, compliance, administrators) should have access.

3. **Data Minimization**: Consider whether all metadata (especially user agent strings) is necessary for your compliance requirements.

4. **User Rights**: Users may have rights to access, correct, or delete their audit log data under privacy regulations.

5. **Error Handling**: This implementation uses a **fail-closed approach** - if audit logging fails, the request fails. This ensures compliance requirements are met but may impact availability if the audit system is down.

## Database Schema

The `audit_logs` table includes:
- Primary key: UUID
- IP addresses stored as VARCHAR(45) to accommodate IPv4 and IPv6 without ports
- Indexed columns for efficient querying:
  - `entity_type`, `entity_id`, `created_at` (composite index)
  - `user_did`, `created_at`
  - `action`, `created_at`

## Input Validation

All logging functions validate inputs:
- **Entity types** must be in the allowed whitelist: `scene`, `event`, `user`, `admin_panel`, `post`
- **Actions** must be in the allowed whitelist: `access_precise_location`, `access_coarse_location`, `view_admin_panel`, etc.
- **Entity IDs** and **actions** cannot be empty
- **Repository** cannot be nil

Invalid inputs return specific errors: `ErrInvalidEntityType`, `ErrInvalidEntityID`, `ErrInvalidAction`, `ErrNilRepository`

## Usage Examples

### Basic Logging with Context

```go
import (
    "github.com/onnwee/subcults/internal/audit"
    "github.com/onnwee/subcults/internal/middleware"
)

// In a handler function with context
func handlePreciseLocationAccess(ctx context.Context, repo audit.Repository) error {
    // Log access to precise location
    err := audit.LogAccess(
        ctx,
        repo,
        "scene",                      // entity type (validated)
        "scene-123",                  // entity ID
        "access_precise_location",    // action (validated)
    )
    if err != nil {
        return err  // Fail-closed: request fails if audit logging fails
    }
    
    // Continue with actual access...
    return nil
}
```

### Logging with HTTP Request Metadata

```go
// In an HTTP handler
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // Log access with full request metadata
    // IP address extraction (consistent with rate limiting):
    // - Checks X-Forwarded-For header first (uses first IP from comma-separated list)
    // - Falls back to X-Real-IP header
    // - Finally uses RemoteAddr (with port stripped for both IPv4 and IPv6)
    err := audit.LogAccessFromRequest(
        r,
        h.auditRepo,
        "admin_panel",
        "privacy_settings",
        "view_admin_panel",
    )
    if err != nil {
        // Handle error - request should fail if audit logging fails
        http.Error(w, "Internal Server Error", http.StatusInternalServerError)
        return
    }
    
    // Continue with handler logic...
}
```

### Querying Audit Logs

```go
// Query by entity (e.g., all access to a specific scene)
logs, err := repo.QueryByEntity("scene", "scene-123", 10) // limit to 10 most recent
if err != nil {
    return err
}

for _, log := range logs {
    fmt.Printf("User %s accessed %s at %s\n", 
        log.UserDID, log.Action, log.CreatedAt)
}

// Query by user (e.g., all access by a specific user)
userLogs, err := repo.QueryByUser("did:web:example.com:user123", 0) // no limit
if err != nil {
    return err
}
```

## Common Actions

Standard action names for consistency:

### Location Access
- `access_precise_location` - Viewing precise coordinates
- `access_coarse_location` - Viewing coarse/fuzzy location

### Administrative
- `view_admin_panel` - Accessing admin interface
- `view_privacy_settings` - Viewing privacy configuration
- `modify_privacy_settings` - Changing privacy settings

### Scene/Event Management
- `view_scene_details` - Viewing scene information
- `view_event_details` - Viewing event information
- `export_member_data` - Exporting user data

## Integration Points

Audit logging should be invoked at:

1. **Precise Location Endpoints** - Any API endpoint that returns precise geographic coordinates
2. **Admin Privacy Panel** - When administrators access privacy-related settings
3. **Data Export** - When user data is exported or downloaded
4. **Permission Changes** - When location consent or privacy settings are modified

## Testing

The package includes comprehensive tests with in-memory repository implementation:

```bash
go test -v ./internal/audit/...
```

## Performance Considerations

- Indexes are created for all common query patterns
- Logs are sorted by time (newest first) in query results
- Use limit parameter in queries to control result size
- Consider implementing retention policies (not yet implemented)

## Future Enhancements

- Retention policy automation (e.g., delete logs older than X days)
- Postgres repository implementation for production use
- Audit log export functionality
- Real-time monitoring/alerting for suspicious access patterns
