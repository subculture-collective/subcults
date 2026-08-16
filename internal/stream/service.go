// Package stream provides the Service layer for stream session business logic.
package stream

import (
	"context"
	"errors"
	"time"
)

// Service orchestrates stream session business logic over the three repository
// interfaces. LiveKit room operations, audit logging, and WebSocket broadcasting
// remain in the HTTP handler layer as infrastructure concerns.
type Service struct {
	sessionRepo     SessionRepository
	participantRepo ParticipantRepository
	analyticsRepo   AnalyticsRepository
	metrics         *Metrics
	now             func() time.Time
}

// NewService creates a new Service with the given repositories and metrics.
func NewService(
	sessionRepo SessionRepository,
	participantRepo ParticipantRepository,
	analyticsRepo AnalyticsRepository,
	metrics *Metrics,
) *Service {
	return &Service{
		sessionRepo:     sessionRepo,
		participantRepo: participantRepo,
		analyticsRepo:   analyticsRepo,
		metrics:         metrics,
		now:             time.Now,
	}
}

// CreateStream creates a new stream session. One of sceneID or eventID must be
// provided. The caller is responsible for ownership validation and LiveKit room
// creation.
func (s *Service) CreateStream(ctx context.Context, sceneID, eventID *string, hostDID string) (*Session, error) {
	id, _, err := s.sessionRepo.CreateStreamSession(sceneID, eventID, hostDID)
	if err != nil {
		return nil, err
	}

	session, err := s.sessionRepo.GetByID(id)
	if err != nil {
		return nil, err
	}

	return session, nil
}

// EndStream ends a stream session and computes analytics. The caller is
// responsible for ownership validation and LiveKit room deletion.
func (s *Service) EndStream(ctx context.Context, streamID string) error {
	if err := s.sessionRepo.EndStreamSession(streamID); err != nil {
		return err
	}

	if s.analyticsRepo != nil {
		if _, err := s.analyticsRepo.ComputeAnalytics(streamID); err != nil {
			return err
		}
	}

	return nil
}

// GetStream retrieves a stream session by ID.
func (s *Service) GetStream(ctx context.Context, streamID string) (*Session, error) {
	return s.sessionRepo.GetByID(streamID)
}

// UpdateStream verifies that a stream session exists. The caller is responsible
// for ownership validation and LiveKit metadata updates.
func (s *Service) UpdateStream(ctx context.Context, streamID string) (*Session, error) {
	return s.sessionRepo.GetByID(streamID)
}

// JoinStream records a participant joining a stream. It generates a participant
// identity from the userDID, records the join in the participant and session
// repositories, records an analytics event, and increments metrics.
// geohashPrefix is an optional 4-character geohash prefix for geographic analytics.
// Returns the participant record and whether this is a reconnection.
func (s *Service) JoinStream(ctx context.Context, streamID, userDID string, geohashPrefix *string) (*Participant, bool, error) {
	participantID := GenerateParticipantID(userDID)

	var participant *Participant
	var isReconnection bool

	if s.participantRepo != nil {
		var err error
		participant, isReconnection, err = s.participantRepo.RecordJoin(streamID, participantID, userDID)
		if err != nil {
			if errors.Is(err, ErrParticipantAlreadyActive) {
				// Duplicate join — log and continue
			} else {
				return nil, false, err
			}
		}
	}

	// Record join count in session
	if err := s.sessionRepo.RecordJoin(streamID); err != nil {
		return nil, false, err
	}

	// Record analytics event
	if s.analyticsRepo != nil {
		_ = s.analyticsRepo.RecordParticipantEvent(streamID, userDID, "join", geohashPrefix)
	}

	// Increment Prometheus counter
	if s.metrics != nil {
		s.metrics.IncStreamJoins()
	}

	return participant, isReconnection, nil
}

// LeaveStream records a participant leaving a stream. It records the leave in
// the participant and session repositories, records an analytics event, and
// increments metrics.
func (s *Service) LeaveStream(ctx context.Context, streamID, userDID string) error {
	participantID := GenerateParticipantID(userDID)

	if s.participantRepo != nil {
		if err := s.participantRepo.RecordLeave(streamID, participantID); err != nil {
			if !errors.Is(err, ErrParticipantNotFound) {
				return err
			}
			// Participant not found or already left — non-fatal
		}
	}

	// Record leave count in session
	if err := s.sessionRepo.RecordLeave(streamID); err != nil {
		return err
	}

	// Record analytics event
	if s.analyticsRepo != nil {
		_ = s.analyticsRepo.RecordParticipantEvent(streamID, userDID, "leave", nil)
	}

	// Increment Prometheus counter
	if s.metrics != nil {
		s.metrics.IncStreamLeaves()
	}

	return nil
}

// GetActiveParticipants returns the active participant count for a stream.
func (s *Service) GetActiveParticipants(ctx context.Context, streamID string) (int, error) {
	if s.participantRepo != nil {
		return s.participantRepo.GetActiveCount(streamID)
	}
	return 0, nil
}

// GetActiveParticipantList returns the list of active participants for a stream.
func (s *Service) GetActiveParticipantList(ctx context.Context, streamID string) ([]*Participant, error) {
	if s.participantRepo != nil {
		return s.participantRepo.GetActiveParticipants(streamID)
	}
	return []*Participant{}, nil
}

// MuteParticipant verifies the stream exists. The caller is responsible for
// ownership validation and LiveKit mute operations.
func (s *Service) MuteParticipant(ctx context.Context, streamID, targetID string) error {
	// Verify stream exists
	_, err := s.sessionRepo.GetByID(streamID)
	return err
}

// KickParticipant removes a participant from the stream data store. The caller
// is responsible for ownership validation and LiveKit removal.
func (s *Service) KickParticipant(ctx context.Context, streamID, targetID string) error {
	if s.participantRepo != nil {
		if err := s.participantRepo.RecordLeave(streamID, targetID); err != nil {
			if !errors.Is(err, ErrParticipantNotFound) {
				return err
			}
		}
	}
	return nil
}

// SetFeaturedParticipant sets or clears the featured participant for a stream.
// The caller is responsible for ownership validation.
func (s *Service) SetFeaturedParticipant(ctx context.Context, streamID string, participantID *string) error {
	return s.sessionRepo.SetFeaturedParticipant(streamID, participantID)
}

// LockStream locks or unlocks a stream. The caller is responsible for ownership
// validation.
func (s *Service) LockStream(ctx context.Context, streamID string, locked bool) error {
	return s.sessionRepo.SetLockStatus(streamID, locked)
}

// GetStreamAnalytics retrieves analytics for a stream session.
func (s *Service) GetStreamAnalytics(ctx context.Context, streamID string) (*Analytics, error) {
	if s.analyticsRepo == nil {
		return nil, ErrAnalyticsNotFound
	}
	return s.analyticsRepo.GetAnalytics(streamID)
}

// SessionRepo exposes the underlying SessionRepository for callers that need
// methods not yet wrapped by the service (e.g., HasActiveStreamForScene).
func (s *Service) SessionRepo() SessionRepository {
	return s.sessionRepo
}

// HasActiveStreamForScene checks if there is an active stream for the given scene.
func (s *Service) HasActiveStreamForScene(sceneID string) (bool, error) {
	return s.sessionRepo.HasActiveStreamForScene(sceneID)
}

// GetActiveStreamForEvent returns the active stream for the given event.
func (s *Service) GetActiveStreamForEvent(eventID string) (*ActiveStreamInfo, error) {
	return s.sessionRepo.GetActiveStreamForEvent(eventID)
}