package identity

import (
	"context"
	"database/sql"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

type memoryLink struct {
	MagicLink
	ConsumedAt *time.Time
}

type memorySession struct {
	ID             string
	UserID         string
	RefreshHash    string
	UserAgentHash  string
	IdleExpiresAt  time.Time
	AbsoluteExpiry time.Time
	RevokedAt      *time.Time
}

type InMemoryRepository struct {
	mu              sync.Mutex
	identities      map[string]EmailIdentity
	identityByHMAC  map[string]string
	links           map[string]memoryLink
	users           map[string]User
	sessions        map[string]memorySession
	creatorRequests map[string]CreatorAccessRequest
}

func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{
		identities:      make(map[string]EmailIdentity),
		identityByHMAC:  make(map[string]string),
		links:           make(map[string]memoryLink),
		users:           make(map[string]User),
		sessions:        make(map[string]memorySession),
		creatorRequests: make(map[string]CreatorAccessRequest),
	}
}

func (r *InMemoryRepository) UpsertEmailIdentity(_ context.Context, encryptedEmail []byte, emailHMAC string) (EmailIdentity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if id := r.identityByHMAC[emailHMAC]; id != "" {
		return r.identities[id], nil
	}
	identity := EmailIdentity{ID: uuid.NewString(), EncryptedEmail: append([]byte(nil), encryptedEmail...), EmailHMAC: emailHMAC}
	r.identities[identity.ID] = identity
	r.identityByHMAC[emailHMAC] = identity.ID
	return identity, nil
}

func (r *InMemoryRepository) CreateMagicLink(_ context.Context, link MagicLink) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.links[link.TokenHash] = memoryLink{MagicLink: link}
	return nil
}

func (r *InMemoryRepository) CompleteMagicLink(_ context.Context, tokenHash, refreshHash, userAgentHash string, now, idleExpiresAt, absoluteExpiresAt time.Time) (SessionResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	link, ok := r.links[tokenHash]
	if !ok || link.ConsumedAt != nil || !link.ExpiresAt.After(now) {
		return SessionResult{}, ErrInvalidToken
	}
	identity := r.identities[link.IdentityID]
	if identity.UserID == "" {
		id := uuid.NewString()
		user := User{
			ID:          id,
			InternalDID: "did:web:subcults.subcult.tv:users:" + id,
			Handle:      "member-" + id[:8],
			Role:        "participant",
		}
		r.users[id] = user
		identity.UserID = id
		identity.VerifiedAt = ptrTime(now)
		r.identities[identity.ID] = identity
	}
	link.ConsumedAt = ptrTime(now)
	r.links[tokenHash] = link
	session := memorySession{ID: uuid.NewString(), UserID: identity.UserID, RefreshHash: refreshHash, UserAgentHash: userAgentHash, IdleExpiresAt: idleExpiresAt, AbsoluteExpiry: absoluteExpiresAt}
	r.sessions[refreshHash] = session
	return SessionResult{User: r.users[identity.UserID], SessionID: session.ID, ReturnPath: link.ReturnPath}, nil
}

func (r *InMemoryRepository) RotateSession(_ context.Context, oldRefreshHash, newRefreshHash, userAgentHash string, now, idleExpiresAt time.Time) (SessionResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	old, ok := r.sessions[oldRefreshHash]
	if !ok || old.RevokedAt != nil || !old.IdleExpiresAt.After(now) || !old.AbsoluteExpiry.After(now) {
		return SessionResult{}, ErrInvalidSession
	}
	old.RevokedAt = ptrTime(now)
	r.sessions[oldRefreshHash] = old
	if idleExpiresAt.After(old.AbsoluteExpiry) {
		idleExpiresAt = old.AbsoluteExpiry
	}
	next := memorySession{ID: uuid.NewString(), UserID: old.UserID, RefreshHash: newRefreshHash, UserAgentHash: userAgentHash, IdleExpiresAt: idleExpiresAt, AbsoluteExpiry: old.AbsoluteExpiry}
	r.sessions[newRefreshHash] = next
	return SessionResult{User: r.users[old.UserID], SessionID: next.ID}, nil
}

func (r *InMemoryRepository) GetUser(_ context.Context, userID string) (User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	user, ok := r.users[userID]
	if !ok {
		return User{}, ErrInvalidSession
	}
	return user, nil
}

func (r *InMemoryRepository) CompleteProfile(_ context.Context, userID, handle, displayName string) (User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, user := range r.users {
		if id != userID && user.Handle == handle {
			return User{}, ErrHandleUnavailable
		}
	}
	user, ok := r.users[userID]
	if !ok {
		return User{}, ErrInvalidSession
	}
	user.Handle = handle
	user.DisplayName = displayName
	user.OnboardingComplete = true
	r.users[userID] = user
	return user, nil
}

func (r *InMemoryRepository) RevokeSession(_ context.Context, refreshHash string, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if session, ok := r.sessions[refreshHash]; ok && session.RevokedAt == nil {
		session.RevokedAt = ptrTime(now)
		r.sessions[refreshHash] = session
	}
	return nil
}

func (r *InMemoryRepository) RevokeAllSessions(_ context.Context, userID string, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for hash, session := range r.sessions {
		if session.UserID == userID && session.RevokedAt == nil {
			session.RevokedAt = ptrTime(now)
			r.sessions[hash] = session
		}
	}
	return nil
}

func ptrTime(value time.Time) *time.Time { return &value }

func (r *InMemoryRepository) CreateCreatorAccessRequest(_ context.Context, userID, statement string, now time.Time) (CreatorAccessRequest, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, request := range r.creatorRequests {
		if request.UserID == userID && request.Status == "pending" {
			return request, nil
		}
	}
	request := CreatorAccessRequest{ID: uuid.NewString(), UserID: userID, Statement: statement, Status: "pending", CreatedAt: now}
	r.creatorRequests[request.ID] = request
	if user, ok := r.users[userID]; ok {
		user.Role = "creator_pending"
		r.users[userID] = user
	}
	return request, nil
}

func (r *InMemoryRepository) GetCreatorAccessRequest(_ context.Context, userID string) (CreatorAccessRequest, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var latest CreatorAccessRequest
	for _, request := range r.creatorRequests {
		if request.UserID == userID && request.CreatedAt.After(latest.CreatedAt) {
			latest = request
		}
	}
	if latest.ID == "" {
		return CreatorAccessRequest{}, sql.ErrNoRows
	}
	return latest, nil
}

func (r *InMemoryRepository) ListCreatorAccessRequests(_ context.Context, status string) ([]CreatorAccessRequest, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	requests := make([]CreatorAccessRequest, 0, len(r.creatorRequests))
	for _, request := range r.creatorRequests {
		if status == "" || request.Status == status {
			requests = append(requests, request)
		}
	}
	sort.Slice(requests, func(i, j int) bool { return requests[i].CreatedAt.Before(requests[j].CreatedAt) })
	return requests, nil
}

func (r *InMemoryRepository) ReviewCreatorAccessRequest(_ context.Context, requestID, reviewerID, status, note string, now time.Time) (CreatorAccessRequest, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	request, ok := r.creatorRequests[requestID]
	if !ok || request.Status != "pending" {
		return CreatorAccessRequest{}, sql.ErrNoRows
	}
	request.Status, request.ReviewNote, request.ReviewedByID, request.ReviewedAt = status, note, reviewerID, ptrTime(now)
	r.creatorRequests[requestID] = request
	user := r.users[request.UserID]
	if status == "approved" {
		user.Role = "creator"
	} else {
		user.Role = "participant"
	}
	r.users[user.ID] = user
	return request, nil
}
