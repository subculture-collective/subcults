// Package audit provides audit logging functionality for tracking access to
// sensitive endpoints and operations for compliance and incident response.
package audit

import (
	"time"
)

// AuditLog represents a single audit event in the system.
type AuditLog struct {
	ID         string
	UserDID    string
	EntityType string
	EntityID   string
	Action     string
	CreatedAt  time.Time

	// Optional metadata
	RequestID string
	IPAddress string
	UserAgent string
}

// LogEntry represents the input for creating an audit log entry.
type LogEntry struct {
	UserDID    string
	EntityType string
	EntityID   string
	Action     string

	// Optional metadata
	RequestID string
	IPAddress string
	UserAgent string
}
