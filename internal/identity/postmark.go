package identity

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

type DevelopmentMailer struct{ Logger *slog.Logger }

func (m DevelopmentMailer) SendMagicLink(_ context.Context, email, verificationURL string) error {
	logger := m.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.Warn("development magic link generated", "email", email, "verification_url", verificationURL)
	return nil
}

type PostmarkMailer struct {
	client      *http.Client
	serverToken string
	from        string
	stream      string
}

func NewPostmarkMailer(serverToken, from, stream string) (*PostmarkMailer, error) {
	if serverToken == "" || from == "" || stream == "" {
		return nil, errors.New("Postmark token, sender, and message stream are required")
	}
	return &PostmarkMailer{
		client:      &http.Client{Timeout: 10 * time.Second},
		serverToken: serverToken,
		from:        from,
		stream:      stream,
	}, nil
}

func (m *PostmarkMailer) SendMagicLink(ctx context.Context, email, verificationURL string) error {
	payload := map[string]any{
		"From":          m.from,
		"To":            email,
		"Subject":       "Your SUBCULT access link",
		"TextBody":      "Enter SUBCULT with this one-time link (valid for 15 minutes):\n\n" + verificationURL,
		"HtmlBody":      `<p>Enter SUBCULT with this one-time link:</p><p><a href="` + verificationURL + `">Open SUBCULT</a></p><p>This link expires in 15 minutes and can only be used once.</p>`,
		"MessageStream": m.stream,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode Postmark request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.postmarkapp.com/email", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create Postmark request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Postmark-Server-Token", m.serverToken)
	response, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("send Postmark request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 2048))
		return fmt.Errorf("Postmark returned %s: %s", response.Status, string(message))
	}
	return nil
}
