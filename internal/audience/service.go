package audience

import (
	"context"
	"time"
)

// Service resolves delivery authorization from separately stored contact, consent, and suppression evidence.
type Service struct {
	repository Repository
}

// NewService constructs an audience authorization service.
func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

// CreateScope persists a reusable delivery scope.
func (s *Service) CreateScope(ctx context.Context, scope DeliveryScope) (string, error) {
	return s.repository.PutScope(ctx, scope)
}

// ScopeIDFor returns an existing scope ID without creating it.
func (s *Service) ScopeIDFor(ctx context.Context, scope DeliveryScope) (string, error) {
	return s.repository.ScopeIDFor(ctx, scope)
}

// AddContact records a private delivery endpoint.
func (s *Service) AddContact(ctx context.Context, contact ContactPoint) error {
	return s.repository.PutContact(ctx, contact)
}

// LinkContact records a revocable DID-to-contact proof.
func (s *Service) LinkContact(ctx context.Context, link ContactPointLink) error {
	return s.repository.PutLink(ctx, link)
}

// ActiveContactsForDID returns only contacts connected by current proof. It is
// the safe bridge from authenticated identity to a private delivery endpoint.
func (s *Service) ActiveContactsForDID(ctx context.Context, did string) ([]ContactPoint, error) {
	return s.repository.ActiveContactsForDID(ctx, did)
}

// GetScope returns the immutable delivery boundary rendered to a participant.
func (s *Service) GetScope(ctx context.Context, scopeID string) (DeliveryScope, error) {
	return s.repository.GetScope(ctx, scopeID)
}

// ConsentStatus returns the effective state for one stored scope without
// treating verification or participation as a grant.
func (s *Service) ConsentStatus(ctx context.Context, contactID, scopeID string) (ConsentAction, bool, error) {
	scope, err := s.repository.GetScope(ctx, scopeID)
	if err != nil {
		return "", false, err
	}
	events, err := s.repository.ApplicableConsent(ctx, contactID, scope)
	if err != nil {
		return "", false, err
	}
	for _, event := range events {
		if event.ScopeID == scopeID {
			return event.Action, true, nil
		}
	}
	return "", false, nil
}

// RecordRelationship records participation evidence without changing delivery authorization.
func (s *Service) RecordRelationship(ctx context.Context, relationship Relationship) error {
	return s.repository.RecordRelationship(ctx, relationship)
}

// Grant records affirmative delivery consent for a stored scope.
func (s *Service) Grant(ctx context.Context, contactID, scopeID, captureSource string, evidence map[string]string, at time.Time) error {
	return s.recordConsent(ctx, contactID, scopeID, ConsentGrant, captureSource, evidence, at)
}

// Revoke records a revoke event for a stored scope.
func (s *Service) Revoke(ctx context.Context, contactID, scopeID, captureSource string, evidence map[string]string, at time.Time) error {
	return s.recordConsent(ctx, contactID, scopeID, ConsentRevoke, captureSource, evidence, at)
}

func (s *Service) recordConsent(ctx context.Context, contactID, scopeID string, action ConsentAction, captureSource string, evidence map[string]string, at time.Time) error {
	if _, err := s.repository.GetContact(ctx, contactID); err != nil {
		return err
	}
	if _, err := s.repository.GetScope(ctx, scopeID); err != nil {
		return err
	}
	return s.repository.RecordConsent(ctx, ConsentEvent{
		ContactPointID: contactID,
		ScopeID:        scopeID,
		Action:         action,
		CaptureSource:  captureSource,
		Evidence:       evidence,
		OccurredAt:     at,
	})
}

// Suppress records a delivery block at global, channel, sender, or scope level.
func (s *Service) Suppress(ctx context.Context, suppression Suppression) error {
	return s.repository.RecordSuppression(ctx, suppression)
}

// CanDeliver authorizes only a verified contact with at least one containing
// grant, no containing revoke, and no applicable suppression. Participation and
// DID-contact links do not themselves grant permission.
func (s *Service) CanDeliver(ctx context.Context, contactID string, request DeliveryScope) (bool, error) {
	if err := request.Validate(); err != nil {
		return false, err
	}
	if repository, ok := s.repository.(interface {
		CanDeliverTransactional(context.Context, string, DeliveryScope) (bool, error)
	}); ok {
		return repository.CanDeliverTransactional(ctx, contactID, request)
	}
	contact, err := s.repository.GetContact(ctx, contactID)
	if err != nil {
		return false, err
	}
	if contact.VerifiedAt == nil {
		return false, nil
	}

	suppressed, err := s.repository.HasApplicableSuppression(ctx, contactID, request)
	if err != nil {
		return false, err
	}
	if suppressed {
		return false, nil
	}

	events, err := s.repository.ApplicableConsent(ctx, contactID, request)
	if err != nil {
		return false, err
	}
	granted := false
	for _, event := range events {
		if event.Action == ConsentRevoke {
			return false, nil
		}
		if event.Action == ConsentGrant {
			granted = true
		}
	}
	return granted, nil
}
