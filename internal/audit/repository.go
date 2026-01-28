package audit

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// Repository defines the interface for audit log operations.
type Repository interface {
	// LogAccess records an access event to the audit log.
	// Returns the created audit log entry.
	LogAccess(entry LogEntry) (*AuditLog, error)

	// QueryByEntity retrieves audit logs for a specific entity, sorted by time (newest first).
	// Limit specifies the maximum number of entries to return (0 = no limit).
	QueryByEntity(entityType, entityID string, limit int) ([]*AuditLog, error)

	// QueryByUser retrieves audit logs for a specific user, sorted by time (newest first).
	// Limit specifies the maximum number of entries to return (0 = no limit).
	QueryByUser(userDID string, limit int) ([]*AuditLog, error)
}

// InMemoryRepository is an in-memory implementation of Repository.
// Used for testing and development. Thread-safe via RWMutex.
type InMemoryRepository struct {
	mu   sync.RWMutex
	logs map[string]*AuditLog
	// Maintain insertion order for queries
	order []string
}

// NewInMemoryRepository creates a new in-memory audit repository.
func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{
		logs:  make(map[string]*AuditLog),
		order: make([]string, 0),
	}
}

// LogAccess records an access event to the audit log.
func (r *InMemoryRepository) LogAccess(entry LogEntry) (*AuditLog, error) {
	log := &AuditLog{
		ID:         uuid.New().String(),
		UserDID:    entry.UserDID,
		EntityType: entry.EntityType,
		EntityID:   entry.EntityID,
		Action:     entry.Action,
		CreatedAt:  time.Now().UTC(),
		RequestID:  entry.RequestID,
		IPAddress:  entry.IPAddress,
		UserAgent:  entry.UserAgent,
	}

	r.mu.Lock()
	r.logs[log.ID] = log
	r.order = append(r.order, log.ID)
	r.mu.Unlock()

	// Return a copy to prevent external modification
	logCopy := *log
	return &logCopy, nil
}

// QueryByEntity retrieves audit logs for a specific entity, sorted by time (newest first).
func (r *InMemoryRepository) QueryByEntity(entityType, entityID string, limit int) ([]*AuditLog, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []*AuditLog

	// Iterate in reverse order (newest first)
	for i := len(r.order) - 1; i >= 0; i-- {
		id := r.order[i]
		log := r.logs[id]

		if log.EntityType == entityType && log.EntityID == entityID {
			// Create a copy to prevent external modification
			logCopy := *log
			results = append(results, &logCopy)

			if limit > 0 && len(results) >= limit {
				break
			}
		}
	}

	return results, nil
}

// QueryByUser retrieves audit logs for a specific user, sorted by time (newest first).
func (r *InMemoryRepository) QueryByUser(userDID string, limit int) ([]*AuditLog, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []*AuditLog

	// Iterate in reverse order (newest first)
	for i := len(r.order) - 1; i >= 0; i-- {
		id := r.order[i]
		log := r.logs[id]

		if log.UserDID == userDID {
			// Create a copy to prevent external modification
			logCopy := *log
			results = append(results, &logCopy)

			if limit > 0 && len(results) >= limit {
				break
			}
		}
	}

	return results, nil
}
