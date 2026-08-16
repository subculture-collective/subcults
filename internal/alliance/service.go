// Package alliance provides models, repository, and service layer for managing
// alliances between scenes with AT Protocol record tracking.
package alliance

import (
	"errors"
	"html"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Service-level sentinel errors used by handlers to produce HTTP-level error
// codes with WriteAPIError.
var (
	ErrInvalidWeight   = errors.New("weight must be between 0.0 and 1.0")
	ErrSelfAlliance    = errors.New("cannot create alliance with same scene")
	ErrEmptySceneIDs   = errors.New("from_scene_id and to_scene_id are required")
	ErrReasonEmpty     = errors.New("reason cannot be empty or whitespace only")
	ErrReasonTooLong   = errors.New("reason must not exceed 256 characters")
)

// MaxReasonLength is the maximum allowed length for alliance reason text.
const MaxReasonLength = 256

// Service holds the alliance repository and a clock for timestamping.
type Service struct {
	repo AllianceRepository
	now  func() time.Time
}

// NewService constructs an alliance domain service.
func NewService(repo AllianceRepository) *Service {
	return &Service{repo: repo, now: time.Now}
}

// ValidateWeight validates alliance weight is between 0.0 and 1.0.
func ValidateWeight(weight float64) error {
	if weight < 0.0 || weight > 1.0 {
		return ErrInvalidWeight
	}
	return nil
}

// ValidateSceneIDs validates that from and to scene IDs are present and not equal.
func ValidateSceneIDs(fromSceneID, toSceneID string) error {
	if strings.TrimSpace(fromSceneID) == "" || strings.TrimSpace(toSceneID) == "" {
		return ErrEmptySceneIDs
	}
	if fromSceneID == toSceneID {
		return ErrSelfAlliance
	}
	return nil
}

// ValidateReason validates alliance reason length and content.
func ValidateReason(reason string) error {
	trimmed := strings.TrimSpace(reason)
	if trimmed == "" {
		return ErrReasonEmpty
	}
	if len(trimmed) > MaxReasonLength {
		return ErrReasonTooLong
	}
	return nil
}

// CreateAlliance creates a new active alliance between two scenes after
// validating inputs and sanitizing the reason. Returns the persisted alliance.
func (s *Service) CreateAlliance(fromSceneID, toSceneID string, weight float64, reason *string) (*Alliance, error) {
	if err := ValidateWeight(weight); err != nil {
		return nil, err
	}
	if err := ValidateSceneIDs(fromSceneID, toSceneID); err != nil {
		return nil, err
	}
	if reason != nil {
		if err := ValidateReason(*reason); err != nil {
			return nil, err
		}
	}

	var sanitizedReason *string
	if reason != nil {
		escaped := html.EscapeString(*reason)
		sanitizedReason = &escaped
	}

	a := &Alliance{
		ID:          uuid.New().String(),
		FromSceneID: fromSceneID,
		ToSceneID:   toSceneID,
		Weight:      weight,
		Status:      "active",
		Reason:      sanitizedReason,
	}

	if err := s.repo.Insert(a); err != nil {
		return nil, err
	}

	return s.repo.GetByID(a.ID)
}

// GetAlliance retrieves an alliance by its ID.
func (s *Service) GetAlliance(id string) (*Alliance, error) {
	return s.repo.GetByID(id)
}

// UpdateAlliance applies partial updates to an existing alliance. Weight and
// reason are optional; nil pointers are left unchanged. Returns the updated
// alliance.
func (s *Service) UpdateAlliance(id string, weight *float64, reason *string) (*Alliance, error) {
	if weight != nil {
		if err := ValidateWeight(*weight); err != nil {
			return nil, err
		}
	}
	if reason != nil {
		if err := ValidateReason(*reason); err != nil {
			return nil, err
		}
	}

	existing, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}

	if weight != nil {
		existing.Weight = *weight
	}
	if reason != nil {
		sanitized := html.EscapeString(*reason)
		existing.Reason = &sanitized
	}

	if err := s.repo.Update(existing); err != nil {
		return nil, err
	}

	return s.repo.GetByID(id)
}

// DeleteAlliance soft-deletes the alliance by its ID.
func (s *Service) DeleteAlliance(id string) error {
	return s.repo.Delete(id)
}