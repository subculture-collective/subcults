package signal

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/onnwee/subcults/internal/audience"
)

type Service struct {
	repository Repository
	now        func() time.Time
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository, now: time.Now}
}

// CreateDraft creates revision one. The audience definition is immutable once a
// signal is published; later material changes make an explicit new revision.
func (s *Service) CreateDraft(ctx context.Context, signal Signal, content Content, audienceDefinition, createdByDID string, publishAt *time.Time) (Revision, error) {
	if signal.CreatedAt.IsZero() {
		signal.CreatedAt = s.now()
	}
	signal.State = StateDraft
	revision := Revision{ID: fmt.Sprintf("%s-r1", signal.ID), SignalID: signal.ID, Number: 1, Content: content, AudienceDefinition: audienceDefinition, PublishAt: publishAt, CreatedByDID: createdByDID, CreatedAt: s.now()}
	return revision, s.repository.CreateSignal(ctx, signal, revision)
}
func (s *Service) Publish(ctx context.Context, signalID string) (Revision, error) {
	revision, err := s.repository.GetLatestRevision(ctx, signalID)
	if err != nil {
		return Revision{}, err
	}
	if err = s.repository.UpdateSignalState(ctx, signalID, StatePublished); err != nil {
		return Revision{}, err
	}
	return revision, nil
}

// Get returns the public signal state with its latest immutable revision.
func (s *Service) Get(ctx context.Context, signalID string) (struct {
	Signal   Signal   `json:"signal"`
	Revision Revision `json:"revision"`
}, error) {
	var result struct {
		Signal   Signal   `json:"signal"`
		Revision Revision `json:"revision"`
	}
	var err error
	if result.Signal, err = s.repository.GetSignal(ctx, signalID); err != nil {
		return result, err
	}
	result.Revision, err = s.repository.GetLatestRevision(ctx, signalID)
	return result, err
}
func (s *Service) ChangeAudience(ctx context.Context, signalID, audienceDefinition, createdByDID string) (Revision, error) {
	latest, err := s.repository.GetLatestRevision(ctx, signalID)
	if err != nil {
		return Revision{}, err
	}
	if latest.AudienceDefinition == audienceDefinition {
		return latest, nil
	}
	supersedes := latest.ID
	next := Revision{ID: fmt.Sprintf("%s-r%d", signalID, latest.Number+1), SignalID: signalID, Number: latest.Number + 1, Content: latest.Content, AudienceDefinition: audienceDefinition, PublishAt: latest.PublishAt, CreatedByDID: createdByDID, CreatedAt: s.now(), Supersedes: &supersedes}
	if err = s.repository.CreateRevision(ctx, next); err != nil {
		return Revision{}, err
	}
	return next, nil
}

// ChangeContent creates a new immutable revision after publication when any
// message material changes. It deliberately never mutates an already queued
// revision, preserving the audience snapshot and delivery provenance.
func (s *Service) ChangeContent(ctx context.Context, signalID string, content Content, createdByDID string) (Revision, error) {
	latest, err := s.repository.GetLatestRevision(ctx, signalID)
	if err != nil {
		return Revision{}, err
	}
	if latest.Content == content {
		return latest, nil
	}
	supersedes := latest.ID
	next := Revision{ID: fmt.Sprintf("%s-r%d", signalID, latest.Number+1), SignalID: signalID, Number: latest.Number + 1, Content: content, AudienceDefinition: latest.AudienceDefinition, PublishAt: latest.PublishAt, CreatedByDID: createdByDID, CreatedAt: s.now(), Supersedes: &supersedes}
	if err = s.repository.CreateRevision(ctx, next); err != nil {
		return Revision{}, err
	}
	return next, nil
}

type AudienceMember struct {
	ContactPointID string
	ToToken        []byte
}

// SnapshotDeliveries sorts contact IDs before persistence and relies on the
// repository uniqueness key to make retried scheduling idempotent.
func (s *Service) SnapshotDeliveries(ctx context.Context, revisionID, channel, purpose, provider string, scope audience.DeliveryScope, members []AudienceMember, scheduledAt time.Time) ([]Delivery, error) {
	if channel != "web_push" && channel != "email" {
		return nil, ErrInvalidChannel
	}
	members = append([]AudienceMember(nil), members...)
	sort.Slice(members, func(i, j int) bool { return members[i].ContactPointID < members[j].ContactPointID })
	result := make([]Delivery, 0, len(members))
	for _, member := range members {
		d := Delivery{ID: fmt.Sprintf("%s-%s-%s", revisionID, channel, member.ContactPointID), SignalRevisionID: revisionID, ContactPointID: member.ContactPointID, Channel: channel, Purpose: purpose, Provider: provider, Scope: scope, ToToken: append([]byte(nil), member.ToToken...), State: DeliveryQueued, ScheduledAt: scheduledAt, UpdatedAt: s.now()}
		stored, err := s.repository.CreateDelivery(ctx, d)
		if err != nil {
			return nil, err
		}
		result = append(result, stored)
	}
	return result, nil
}
