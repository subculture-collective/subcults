package atprotocol

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bluesky-social/indigo/atproto/atclient"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/redis/go-redis/v9"
)

var (
	ErrPublicationScope = errors.New("AT Protocol publication permission is required")
	ErrEntityForbidden  = errors.New("entity is not owned by the authenticated user")
	ErrRecordConflict   = errors.New("AT Protocol record changed; refresh before publishing")
)

// RecordMapping tracks a canonical PDS record and its local projection state.
type RecordMapping struct {
	EntityType       string    `json:"entity_type"`
	EntityID         string    `json:"entity_id"`
	PublisherDID     string    `json:"publisher_did"`
	Collection       string    `json:"collection"`
	RKey             string    `json:"rkey"`
	ATURI            string    `json:"at_uri"`
	CID              string    `json:"cid,omitempty"`
	ProjectionStatus string    `json:"projection_status"`
	RecordVersion    int64     `json:"record_version"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// PublicationResult is returned after the authoritative PDS write succeeds.
type PublicationResult struct {
	ATURI            string `json:"at_uri"`
	CID              string `json:"cid"`
	ProjectionStatus string `json:"projection_status"`
	RecordVersion    int64  `json:"record_version"`
}

// PublicationLocker serializes refresh-token and record-CID mutation.
type PublicationLocker interface {
	WithLock(context.Context, string, func() error) error
}

// RedisPublicationLocker provides a distributed per-session lock.
type RedisPublicationLocker struct {
	client *redis.Client
	ttl    time.Duration
}

// NewRedisPublicationLocker creates a bounded distributed lock.
func NewRedisPublicationLocker(client *redis.Client) *RedisPublicationLocker {
	return &RedisPublicationLocker{client: client, ttl: 30 * time.Second}
}

// WithLock runs fn while holding a unique Redis lease.
func (l *RedisPublicationLocker) WithLock(ctx context.Context, key string, fn func() error) error {
	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		return err
	}
	token := hex.EncodeToString(tokenBytes)
	lockKey := "subcults:atproto:lock:" + key
	deadline := time.Now().Add(5 * time.Second)
	for {
		acquired, err := l.client.SetNX(ctx, lockKey, token, l.ttl).Result()
		if err != nil {
			return fmt.Errorf("acquire AT Protocol session lock: %w", err)
		}
		if acquired {
			break
		}
		if time.Now().After(deadline) {
			return errors.New("AT Protocol session is busy")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
	defer func() {
		const release = "if redis.call('get',KEYS[1]) == ARGV[1] then return redis.call('del',KEYS[1]) else return 0 end"
		_ = l.client.Eval(context.Background(), release, []string{lockKey}, token).Err()
	}()
	return fn()
}

// SetPublicationLocker configures the required cross-instance session lock.
func (s *OAuthService) SetPublicationLocker(locker PublicationLocker) {
	s.publicationLocker = locker
}

// Publish writes a validated public record to the linked creator's PDS.
func (s *OAuthService) Publish(ctx context.Context, userID, entityType, entityID, collection string, payload []byte, swapCID string) (PublicationResult, error) {
	if s.publicationLocker == nil {
		return PublicationResult{}, errors.New("AT Protocol publication lock is unavailable")
	}
	if err := ValidatePublicRecord(collection, payload); err != nil {
		return PublicationResult{}, err
	}
	if !entityCollectionMatches(entityType, collection) {
		return PublicationResult{}, ErrUnsupportedCollection
	}
	link, err := s.store.LinkForUser(ctx, userID)
	if err != nil {
		return PublicationResult{}, err
	}
	if link.Status != "active" || !containsScope(link.GrantedScopes, "repo:"+collection) {
		return PublicationResult{}, ErrPublicationScope
	}
	mapping, err := s.store.ReserveRecord(ctx, userID, entityType, entityID, collection, link.AccountDID, s.tidClock.Next().String())
	if err != nil {
		return PublicationResult{}, err
	}
	var record map[string]any
	if err := json.Unmarshal(payload, &record); err != nil {
		return PublicationResult{}, ErrInvalidRecord
	}
	var output struct {
		URI string `json:"uri"`
		CID string `json:"cid"`
	}
	err = s.publicationLocker.WithLock(ctx, link.AccountDID+":"+link.SessionID, func() error {
		session, err := s.publishApp.ResumeSession(ctx, syntax.DID(link.AccountDID), link.SessionID)
		if err != nil {
			return err
		}
		body := map[string]any{
			"repo": link.AccountDID, "collection": collection,
			"rkey": mapping.RKey, "record": record,
		}
		endpoint := syntax.NSID("com.atproto.repo.createRecord")
		if mapping.CID != "" {
			if swapCID == "" || swapCID != mapping.CID {
				return ErrRecordConflict
			}
			body["swapRecord"] = swapCID
			endpoint = syntax.NSID("com.atproto.repo.putRecord")
		}
		return session.APIClient().Post(ctx, endpoint, body, &output)
	})
	if err != nil {
		var apiErr *atclient.APIError
		if errors.As(err, &apiErr) && (apiErr.StatusCode == 409 || strings.Contains(strings.ToLower(apiErr.Name), "swap")) {
			err = ErrRecordConflict
		}
		if !errors.Is(err, ErrRecordConflict) {
			_ = s.store.MarkRecordFailed(ctx, mapping.ATURI)
		}
		return PublicationResult{}, err
	}
	if output.URI != mapping.ATURI || strings.TrimSpace(output.CID) == "" {
		_ = s.store.MarkRecordFailed(ctx, mapping.ATURI)
		return PublicationResult{}, ErrRecordConflict
	}
	updated, err := s.store.MarkRecordAwaiting(ctx, mapping.ATURI, output.CID, mapping.RecordVersion)
	if err != nil {
		// The PDS write is canonical and must not be reported as rolled back just
		// because the local projection marker is delayed. Reconciliation can
		// fetch the stable URI directly from the authoritative repository.
		return PublicationResult{ATURI: output.URI, CID: output.CID,
			ProjectionStatus: "awaiting_projection", RecordVersion: mapping.RecordVersion}, nil
	}
	return PublicationResult{
		ATURI: output.URI, CID: output.CID,
		ProjectionStatus: updated.ProjectionStatus,
		RecordVersion:    updated.RecordVersion,
	}, nil
}

// PublishEntity builds a disclosure-safe portable record on the server before
// writing it to the creator's PDS.
func (s *OAuthService) PublishEntity(ctx context.Context, userID, entityType, entityID, swapCID string) (PublicationResult, error) {
	collection, payload, err := s.store.PublicRecordForEntity(ctx, userID, strings.ToLower(strings.TrimSpace(entityType)), entityID)
	if err != nil {
		return PublicationResult{}, err
	}
	return s.Publish(ctx, userID, strings.ToLower(strings.TrimSpace(entityType)), entityID, collection, payload, swapCID)
}

// Projection returns safe indexing status by canonical AT URI.
func (s *OAuthService) Projection(ctx context.Context, atURI string) (RecordMapping, error) {
	return s.store.RecordByURI(ctx, atURI)
}

func containsScope(scopes []string, expected string) bool {
	for _, scope := range scopes {
		if scope == expected {
			return true
		}
	}
	return false
}

func entityCollectionMatches(entityType, collection string) bool {
	expected := map[string]string{
		"profile": CollectionProfile, "act": CollectionAct, "place": CollectionPlace,
		"venue": CollectionVenue, "scene": CollectionScene, "event": CollectionEvent, "tour": CollectionTour,
		"appearance": CollectionAppearance, "assertion": CollectionAssertion,
	}
	return expected[strings.ToLower(entityType)] == collection
}
