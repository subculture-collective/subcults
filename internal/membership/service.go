// Package membership provides models, repository, and service layer for
// managing scene membership requests with AT Protocol record tracking.
package membership

import (
	"errors"
	"time"
)

// Service-level sentinel errors used by handlers to produce HTTP-level error
// codes with WriteAPIError.
var (
	ErrAlreadyActiveMember  = errors.New("user is already an active member")
	ErrPendingRequestExists = errors.New("pending membership request already exists")
	ErrNotPending           = errors.New("only pending membership requests can be updated")
)

// Service holds the membership repository and a clock for timestamping.
type Service struct {
	repo MembershipRepository
	now  func() time.Time
}

// NewService constructs a membership domain service.
func NewService(repo MembershipRepository) *Service {
	return &Service{repo: repo, now: time.Now}
}

// CreateRequest creates a pending membership request for the given user in the
// given scene. Rejected memberships are re-opened as a new pending request by
// re-using the existing membership ID. Returns the created membership with
// timestamps populated.
func (s *Service) CreateRequest(sceneID, userDID string) (*Membership, error) {
	// Check existing membership
	existing, err := s.repo.GetBySceneAndUser(sceneID, userDID)
	if err != nil && !errors.Is(err, ErrMembershipNotFound) {
		return nil, err
	}

	if existing != nil {
		switch existing.Status {
		case "pending":
			return nil, ErrPendingRequestExists
		case "active":
			return nil, ErrAlreadyActiveMember
		// rejected: allow re-application by updating the existing record
		}
	}

	// Build the membership
	m := &Membership{
		SceneID:     sceneID,
		UserDID:     userDID,
		Role:        "member",
		Status:      "pending",
		TrustWeight: 0.5,
	}

	// Re-use the rejected membership ID to replace the old record
	if existing != nil && existing.Status == "rejected" {
		m.ID = existing.ID
	}

	result, err := s.repo.Upsert(m)
	if err != nil {
		return nil, err
	}

	return s.repo.GetByID(result.ID)
}

// GetRequest retrieves a membership by its ID.
func (s *Service) GetRequest(id string) (*Membership, error) {
	return s.repo.GetByID(id)
}

// ListRequests retrieves all memberships for a scene, optionally filtered by status.
func (s *Service) ListRequests(sceneID, status string) ([]*Membership, error) {
	return s.repo.ListByScene(sceneID, status)
}

// UpdateRequestStatus updates the status of a pending membership request.
// Approving ("active") sets the since timestamp to now. Rejecting ("rejected")
// leaves the since timestamp unchanged. Returns the updated membership.
func (s *Service) UpdateRequestStatus(sceneID, userDID, status string) (*Membership, error) {
	existing, err := s.repo.GetBySceneAndUser(sceneID, userDID)
	if err != nil {
		return nil, err
	}

	if existing.Status != "pending" {
		return nil, ErrNotPending
	}

	var since *time.Time
	if status == "active" {
		now := s.now()
		since = &now
	}

	if err := s.repo.UpdateStatus(existing.ID, status, since); err != nil {
		return nil, err
	}

	return s.repo.GetByID(existing.ID)
}