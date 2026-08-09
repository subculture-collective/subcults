// Package audience owns private relationship evidence and scoped delivery
// authorization. It deliberately separates participation from messaging consent.
package audience

import (
	"errors"
	"strings"
	"time"
)

var (
	// ErrInvalidScope indicates that a delivery scope is missing its required sender, channel, or purpose.
	ErrInvalidScope = errors.New("invalid delivery scope")
	// ErrContactNotFound indicates that no known contact point has the supplied ID.
	ErrContactNotFound = errors.New("contact point not found")
	// ErrScopeNotFound indicates that no known consent scope has the supplied ID.
	ErrScopeNotFound = errors.New("consent scope not found")
)

// ContactPoint is a private delivery endpoint. Value is deliberately absent: production persistence stores an encrypted value and HMAC only.
type ContactPoint struct {
	ID   string
	Kind string
	// EncryptedValue and ValueHMAC are persistence-only material. Plain contact
	// values never cross the repository boundary.
	EncryptedValue []byte
	ValueHMAC      string
	VerifiedAt     *time.Time
}

// ContactPointLink proves a revocable relationship between a DID and a contact point.
type ContactPointLink struct {
	ContactPointID string
	UserDID        string
	VerifiedAt     time.Time
	RevokedAt      *time.Time
}

// Relationship records participation evidence. It never authorizes delivery.
type Relationship struct {
	SubjectDID     string
	ContactPointID string
	ProgramType    string
	ProgramID      string
	Kind           string
	OccurredAt     time.Time
}

// DeliveryScope expresses the sender, channel, purpose, and optional occurrence restrictions for a delivery.
type DeliveryScope struct {
	SenderType        string
	SenderID          string
	Channel           string
	Purpose           string
	TourID            string
	EventID           string
	AppearanceID      string
	PlaceID           string
	DisclosureVersion string
	Region            string
}

// Validate ensures the scope can identify a sender and a delivery purpose.
func (s DeliveryScope) Validate() error {
	if strings.TrimSpace(s.SenderType) == "" || strings.TrimSpace(s.SenderID) == "" ||
		strings.TrimSpace(s.Channel) == "" || strings.TrimSpace(s.Purpose) == "" ||
		strings.TrimSpace(s.DisclosureVersion) == "" {
		return ErrInvalidScope
	}
	return nil
}

// Contains reports whether this stored scope authorizes or revokes the supplied request.
// Sender, channel, and purpose are exact; stored optional restrictions narrow the scope.
func (s DeliveryScope) Contains(request DeliveryScope) bool {
	if s.SenderType != request.SenderType || s.SenderID != request.SenderID ||
		s.Channel != request.Channel || s.Purpose != request.Purpose ||
		s.DisclosureVersion != request.DisclosureVersion {
		return false
	}
	return optionalRestrictionMatches(s.TourID, request.TourID) &&
		optionalRestrictionMatches(s.EventID, request.EventID) &&
		optionalRestrictionMatches(s.AppearanceID, request.AppearanceID) &&
		optionalRestrictionMatches(s.PlaceID, request.PlaceID) &&
		optionalRestrictionMatches(s.Region, request.Region)
}

func optionalRestrictionMatches(stored, request string) bool {
	return stored == "" || stored == request
}

// ConsentAction is an append-only authorization transition.
type ConsentAction string

const (
	ConsentGrant  ConsentAction = "grant"
	ConsentRevoke ConsentAction = "revoke"
)

// ConsentEvent records a grant or revoke for a contact and a reusable scope.
type ConsentEvent struct {
	ContactPointID string
	ScopeID        string
	Action         ConsentAction
	CaptureSource  string
	Evidence       map[string]string
	OccurredAt     time.Time
}

// SuppressionLevel determines how broadly a suppression prevents delivery.
type SuppressionLevel string

const (
	SuppressionGlobal  SuppressionLevel = "global"
	SuppressionChannel SuppressionLevel = "channel"
	SuppressionSender  SuppressionLevel = "sender"
	SuppressionScope   SuppressionLevel = "scope"
)

// Suppression prevents delivery until lifted. Scope suppressions use the stored scope's containment rules.
type Suppression struct {
	ContactPointID string
	Level          SuppressionLevel
	Channel        string
	SenderType     string
	SenderID       string
	ScopeID        string
	OccurredAt     time.Time
	LiftedAt       *time.Time
}
