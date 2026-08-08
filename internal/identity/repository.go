package identity

import (
	"context"
	"time"
)

type Repository interface {
	UpsertEmailIdentity(ctx context.Context, encryptedEmail []byte, emailHMAC string) (EmailIdentity, error)
	CreateMagicLink(ctx context.Context, link MagicLink) error
	CompleteMagicLink(ctx context.Context, tokenHash, refreshHash, userAgentHash string, now, idleExpiresAt, absoluteExpiresAt time.Time) (SessionResult, error)
	RotateSession(ctx context.Context, oldRefreshHash, newRefreshHash, userAgentHash string, now, idleExpiresAt time.Time) (SessionResult, error)
	GetUser(ctx context.Context, userID string) (User, error)
	CompleteProfile(ctx context.Context, userID, handle, displayName string) (User, error)
	RevokeSession(ctx context.Context, refreshHash string, now time.Time) error
	RevokeAllSessions(ctx context.Context, userID string, now time.Time) error
	CreateCreatorAccessRequest(ctx context.Context, userID, statement string, now time.Time) (CreatorAccessRequest, error)
	GetCreatorAccessRequest(ctx context.Context, userID string) (CreatorAccessRequest, error)
	ReviewCreatorAccessRequest(ctx context.Context, requestID, reviewerID, status, note string, now time.Time) (CreatorAccessRequest, error)
}

type Mailer interface {
	SendMagicLink(ctx context.Context, email, verificationURL string) error
}
