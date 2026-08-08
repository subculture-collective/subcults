package audience

import (
	"context"
	"sort"
	"sync"
)

// Repository persists the minimum evidence required to resolve delivery authorization.
type Repository interface {
	PutContact(context.Context, ContactPoint) error
	GetContact(context.Context, string) (ContactPoint, error)
	PutLink(context.Context, ContactPointLink) error
	RecordRelationship(context.Context, Relationship) error
	PutScope(context.Context, DeliveryScope) (string, error)
	ScopeIDFor(context.Context, DeliveryScope) (string, error)
	GetScope(context.Context, string) (DeliveryScope, error)
	RecordConsent(context.Context, ConsentEvent) error
	ApplicableConsent(context.Context, string, DeliveryScope) ([]ConsentEvent, error)
	RecordSuppression(context.Context, Suppression) error
	HasApplicableSuppression(context.Context, string, DeliveryScope) (bool, error)
}

// InMemoryRepository is a thread-safe repository for tests and local fixtures.
type InMemoryRepository struct {
	mu            sync.RWMutex
	contacts      map[string]ContactPoint
	links         []ContactPointLink
	relationships []Relationship
	scopes        map[string]DeliveryScope
	scopeIDs      map[string]string
	consents      []ConsentEvent
	suppressions  []Suppression
}

// NewInMemoryRepository constructs an empty in-memory audience repository.
func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{
		contacts: make(map[string]ContactPoint),
		scopes:   make(map[string]DeliveryScope),
		scopeIDs: make(map[string]string),
	}
}

func (r *InMemoryRepository) PutContact(ctx context.Context, contact ContactPoint) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.contacts[contact.ID] = contact
	return nil
}

func (r *InMemoryRepository) GetContact(ctx context.Context, id string) (ContactPoint, error) {
	if err := ctx.Err(); err != nil {
		return ContactPoint{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	contact, ok := r.contacts[id]
	if !ok {
		return ContactPoint{}, ErrContactNotFound
	}
	return contact, nil
}

func (r *InMemoryRepository) PutLink(ctx context.Context, link ContactPointLink) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.links = append(r.links, link)
	return nil
}

func (r *InMemoryRepository) RecordRelationship(ctx context.Context, relationship Relationship) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.relationships = append(r.relationships, relationship)
	return nil
}

func (r *InMemoryRepository) PutScope(ctx context.Context, scope DeliveryScope) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := scope.Validate(); err != nil {
		return "", err
	}
	key := scopeKey(scope)
	r.mu.Lock()
	defer r.mu.Unlock()
	if id, ok := r.scopeIDs[key]; ok {
		return id, nil
	}
	id := key
	r.scopeIDs[key] = id
	r.scopes[id] = scope
	return id, nil
}

func (r *InMemoryRepository) ScopeIDFor(ctx context.Context, scope DeliveryScope) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.scopeIDs[scopeKey(scope)]
	if !ok {
		return "", ErrScopeNotFound
	}
	return id, nil
}

func (r *InMemoryRepository) GetScope(ctx context.Context, id string) (DeliveryScope, error) {
	if err := ctx.Err(); err != nil {
		return DeliveryScope{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	scope, ok := r.scopes[id]
	if !ok {
		return DeliveryScope{}, ErrScopeNotFound
	}
	return scope, nil
}

func (r *InMemoryRepository) RecordConsent(ctx context.Context, event ConsentEvent) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	copy := event
	copy.Evidence = cloneEvidence(event.Evidence)
	r.consents = append(r.consents, copy)
	return nil
}

func cloneEvidence(evidence map[string]string) map[string]string {
	if evidence == nil {
		return nil
	}
	copy := make(map[string]string, len(evidence))
	for key, value := range evidence {
		copy[key] = value
	}
	return copy
}

func (r *InMemoryRepository) ApplicableConsent(ctx context.Context, contactID string, request DeliveryScope) ([]ConsentEvent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()

	latest := make(map[string]ConsentEvent)
	for _, event := range r.consents {
		if event.ContactPointID != contactID {
			continue
		}
		scope, ok := r.scopes[event.ScopeID]
		if !ok || !scope.Contains(request) {
			continue
		}
		if previous, ok := latest[event.ScopeID]; !ok || event.OccurredAt.After(previous.OccurredAt) ||
			(event.OccurredAt.Equal(previous.OccurredAt) && event.Action == ConsentRevoke) {
			latest[event.ScopeID] = event
		}
	}
	result := make([]ConsentEvent, 0, len(latest))
	for _, event := range latest {
		result = append(result, event)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ScopeID < result[j].ScopeID })
	return result, nil
}

func (r *InMemoryRepository) RecordSuppression(ctx context.Context, suppression Suppression) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.suppressions = append(r.suppressions, suppression)
	return nil
}

func (r *InMemoryRepository) HasApplicableSuppression(ctx context.Context, contactID string, request DeliveryScope) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, suppression := range r.suppressions {
		if suppression.ContactPointID != contactID || suppression.LiftedAt != nil {
			continue
		}
		switch suppression.Level {
		case SuppressionGlobal:
			return true, nil
		case SuppressionChannel:
			if suppression.Channel == request.Channel {
				return true, nil
			}
		case SuppressionSender:
			if suppression.SenderType == request.SenderType && suppression.SenderID == request.SenderID {
				return true, nil
			}
		case SuppressionScope:
			scope, ok := r.scopes[suppression.ScopeID]
			if ok && scope.Contains(request) {
				return true, nil
			}
		}
	}
	return false, nil
}

func scopeKey(scope DeliveryScope) string {
	return scope.SenderType + "\x00" + scope.SenderID + "\x00" + scope.Channel + "\x00" + scope.Purpose + "\x00" +
		scope.TourID + "\x00" + scope.EventID + "\x00" + scope.AppearanceID + "\x00" + scope.PlaceID + "\x00" +
		scope.DisclosureVersion + "\x00" + scope.Region
}
