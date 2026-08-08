// Package signal owns versioned outbound notices and their consented delivery ledger.
package signal

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/onnwee/subcults/internal/audience"
)

var (
	ErrInvalidSignal    = errors.New("invalid signal")
	ErrInvalidChannel   = errors.New("unsupported delivery channel")
	ErrSignalNotFound   = errors.New("signal not found")
	ErrRevisionNotFound = errors.New("signal revision not found")
	ErrDeliveryNotFound = errors.New("delivery not found")
	ErrSuppressed       = errors.New("delivery suppressed or unauthorized")
	ErrProvider         = errors.New("provider delivery failed")
)

type State string

const (
	StateDraft     State = "draft"
	StateScheduled State = "scheduled"
	StatePublished State = "published"
	StateCompleted State = "completed"
	StateCancelled State = "cancelled"
)

type DeliveryState string

const (
	DeliveryQueued     DeliveryState = "queued"
	DeliverySuppressed DeliveryState = "suppressed"
	DeliverySent       DeliveryState = "sent"
	DeliveryDelivered  DeliveryState = "delivered"
	DeliveryFailed     DeliveryState = "failed"
	DeliveryCancelled  DeliveryState = "cancelled"
)

type Signal struct {
	ID              string    `json:"id"`
	OwnerType       string    `json:"owner_type"`
	OwnerID         string    `json:"owner_id"`
	TargetType      string    `json:"target_type"`
	TargetID        string    `json:"target_id"`
	ConsentScopeIDs []string  `json:"consent_scope_ids,omitempty"`
	State           State     `json:"state"`
	CreatedAt       time.Time `json:"created_at"`
}

func (s Signal) Validate() error {
	validTarget := map[string]bool{"scene": true, "profile": true, "event": true, "appearance": true, "tour": true, "post": true, "stream": true, "offer": true}
	if strings.TrimSpace(s.ID) == "" || (s.OwnerType != "scene" && s.OwnerType != "profile") || strings.TrimSpace(s.OwnerID) == "" || strings.TrimSpace(s.TargetID) == "" || !validTarget[s.TargetType] {
		return ErrInvalidSignal
	}
	return nil
}

type Content struct {
	Subject  string `json:"subject"`
	Body     string `json:"body"`
	DeepLink string `json:"deep_link,omitempty"`
}

func (c Content) Validate() error {
	if strings.TrimSpace(c.Body) == "" {
		return ErrInvalidSignal
	}
	return nil
}

type Revision struct {
	ID, SignalID       string
	Number             int
	Content            Content
	AudienceDefinition string
	PublishAt          *time.Time
	CreatedByDID       string
	CreatedAt          time.Time
	Supersedes         *string
}

func (r Revision) Validate() error {
	if strings.TrimSpace(r.SignalID) == "" || r.Number < 1 || strings.TrimSpace(r.AudienceDefinition) == "" || strings.TrimSpace(r.CreatedByDID) == "" {
		return ErrInvalidSignal
	}
	if !json.Valid([]byte(r.AudienceDefinition)) {
		return ErrInvalidSignal
	}
	return r.Content.Validate()
}

type Delivery struct {
	ID, SignalRevisionID, ContactPointID, Channel, Purpose, Provider string
	Scope                                                            audience.DeliveryScope
	ToToken                                                          []byte
	State                                                            DeliveryState
	ProviderMessageID                                                string
	ScheduledAt, UpdatedAt                                           time.Time
}

func (d Delivery) Validate() error {
	if d.ID == "" || d.SignalRevisionID == "" || d.ContactPointID == "" || (d.Channel != "web_push" && d.Channel != "email") || d.Provider == "" || d.Purpose == "" || d.ScheduledAt.IsZero() {
		return ErrInvalidSignal
	}
	return d.Scope.Validate()
}

type Message struct {
	Channel                 string
	ToToken                 []byte
	Subject, Body, DeepLink string
}
type Provider interface {
	Send(ctx context.Context, message Message) (providerMessageID string, err error)
}
