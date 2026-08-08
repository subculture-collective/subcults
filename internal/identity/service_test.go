package identity

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/onnwee/subcults/internal/auth"
)

type captureMailer struct {
	email string
	url   string
}

func (m *captureMailer) SendMagicLink(_ context.Context, email, verificationURL string) error {
	m.email, m.url = email, verificationURL
	return nil
}

func newTestService(t *testing.T) (*Service, *captureMailer) {
	t.Helper()
	protector, err := NewContactProtector([]byte("12345678901234567890123456789012"), []byte("abcdefghijklmnopqrstuvwxyz123456"))
	if err != nil {
		t.Fatal(err)
	}
	mailer := &captureMailer{}
	service, err := NewService(NewInMemoryRepository(), mailer, protector, auth.NewJWTService(strings.Repeat("s", 32)), "https://subcults.example")
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC) }
	return service, mailer
}

func TestMagicLinkCreatesOneTimeSessionAndRotates(t *testing.T) {
	service, mailer := newTestService(t)
	counter := 0
	service.random = func(_ int) (string, error) {
		counter++
		return []string{"magic-token", "refresh-one", "unused-replay-token", "refresh-two"}[counter-1], nil
	}
	if err := service.RequestMagicLink(context.Background(), " PERSON@Example.COM ", "/events/one?from=mail"); err != nil {
		t.Fatal(err)
	}
	if mailer.email != "person@example.com" || !strings.Contains(mailer.url, "magic-token") {
		t.Fatalf("mail = %q %q", mailer.email, mailer.url)
	}
	result, err := service.VerifyMagicLink(context.Background(), "magic-token", "test-agent")
	if err != nil {
		t.Fatal(err)
	}
	if result.User.Role != "participant" || result.ReturnPath != "/events/one?from=mail" || result.AccessToken == "" {
		t.Fatalf("result = %+v", result)
	}
	if _, err := service.VerifyMagicLink(context.Background(), "magic-token", "test-agent"); err != ErrInvalidToken {
		t.Fatalf("replay error = %v", err)
	}
	refreshed, err := service.Refresh(context.Background(), "refresh-one", "test-agent")
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.RefreshToken != "refresh-two" || refreshed.AccessToken == "" {
		t.Fatalf("refresh = %+v", refreshed)
	}
}

func TestMagicLinkRejectsExternalReturnURL(t *testing.T) {
	service, _ := newTestService(t)
	if err := service.RequestMagicLink(context.Background(), "person@example.com", "https://attacker.example"); err != ErrInvalidReturnPath {
		t.Fatalf("error = %v", err)
	}
}

func TestContactProtectorEncryptsAndIndexes(t *testing.T) {
	protector, err := NewContactProtector([]byte("12345678901234567890123456789012"), []byte("abcdefghijklmnopqrstuvwxyz123456"))
	if err != nil {
		t.Fatal(err)
	}
	first, firstHMAC, err := protector.Protect("person@example.com")
	if err != nil {
		t.Fatal(err)
	}
	second, secondHMAC, err := protector.Protect("person@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if string(first) == string(second) || firstHMAC != secondHMAC {
		t.Fatal("encryption must be randomized while lookup HMAC remains stable")
	}
	value, err := protector.Reveal(first)
	if err != nil || value != "person@example.com" {
		t.Fatalf("reveal = %q, %v", value, err)
	}
}
