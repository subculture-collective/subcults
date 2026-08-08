package notification

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/onnwee/subcults/internal/identity"
)

type Keys struct {
	P256DH string `json:"p256dh"`
	Auth   string `json:"auth"`
}
type BrowserSubscription struct {
	Endpoint string `json:"endpoint"`
	Keys     Keys   `json:"keys"`
}
type Payload struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url,omitempty"`
	Tag   string `json:"tag,omitempty"`
}

type Service struct {
	repository                         Repository
	protector                          *identity.ContactProtector
	now                                func() time.Time
	vapidPublic, vapidPrivate, subject string
}

func NewService(repository Repository, protector *identity.ContactProtector, vapidPublic, vapidPrivate, subject string) *Service {
	return &Service{repository: repository, protector: protector, now: time.Now, vapidPublic: strings.TrimSpace(vapidPublic), vapidPrivate: strings.TrimSpace(vapidPrivate), subject: strings.TrimSpace(subject)}
}

func (s *Service) Subscribe(ctx context.Context, userID, userAgent string, input BrowserSubscription) error {
	parsed, err := url.Parse(input.Endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || len(input.Endpoint) > 4096 || input.Keys.P256DH == "" || input.Keys.Auth == "" {
		return errors.New("invalid browser subscription")
	}
	endpoint, endpointID, err := s.protector.Protect(input.Endpoint)
	if err != nil {
		return err
	}
	p256dh, _, err := s.protector.Protect(input.Keys.P256DH)
	if err != nil {
		return err
	}
	auth, _, err := s.protector.Protect(input.Keys.Auth)
	if err != nil {
		return err
	}
	return s.repository.Upsert(ctx, Subscription{UserID: userID, Endpoint: endpoint, EndpointID: endpointID, P256DH: p256dh, Auth: auth, UserAgent: identity.HashToken(userAgent)}, s.now())
}

func (s *Service) Unsubscribe(ctx context.Context, userID, endpoint string) error {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || len(endpoint) > 4096 {
		return errors.New("invalid browser subscription")
	}
	_, endpointID, err := s.protector.Protect(endpoint)
	if err != nil {
		return err
	}
	return s.repository.Revoke(ctx, userID, endpointID, s.now())
}

func (s *Service) Send(ctx context.Context, userID string, payload Payload) error {
	if s.vapidPublic == "" || s.vapidPrivate == "" || s.subject == "" {
		return errors.New("web push delivery is not configured")
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	items, err := s.repository.ListActive(ctx, userID)
	if err != nil {
		return err
	}
	for _, item := range items {
		endpoint, err := s.protector.Reveal(item.Endpoint)
		if err != nil {
			return err
		}
		p256dh, err := s.protector.Reveal(item.P256DH)
		if err != nil {
			return err
		}
		auth, err := s.protector.Reveal(item.Auth)
		if err != nil {
			return err
		}
		response, err := webpush.SendNotificationWithContext(ctx, encoded, &webpush.Subscription{Endpoint: endpoint, Keys: webpush.Keys{P256dh: p256dh, Auth: auth}}, &webpush.Options{Subscriber: s.subject, VAPIDPublicKey: s.vapidPublic, VAPIDPrivateKey: s.vapidPrivate, TTL: 300})
		if response != nil {
			response.Body.Close()
		}
		if err != nil {
			return err
		}
	}
	return nil
}
