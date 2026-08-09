// Command pds-provisioner isolates the PDS administrator credential from the
// public API and exposes only single-use invitation issuance.
package main

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

type server struct {
	pdsURL    string
	adminPass string
	token     string
	client    *http.Client
}

func main() {
	pdsURL := strings.TrimRight(strings.TrimSpace(os.Getenv("PDS_URL")), "/")
	adminPass := strings.TrimSpace(os.Getenv("PDS_ADMIN_PASSWORD"))
	token := strings.TrimSpace(os.Getenv("PDS_PROVISIONER_TOKEN"))
	if pdsURL == "" || adminPass == "" || len(token) < 32 {
		slog.Error("PDS_URL, PDS_ADMIN_PASSWORD, and a 32-character PDS_PROVISIONER_TOKEN are required")
		os.Exit(1)
	}
	srv := &server{
		pdsURL: pdsURL, adminPass: adminPass, token: token,
		client: &http.Client{Timeout: 8 * time.Second},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health/live", srv.health)
	mux.HandleFunc("/v1/invites", srv.invites)
	httpServer := &http.Server{
		Addr: ":8090", Handler: mux, ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second,
		IdleTimeout: 30 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()
	slog.Info("PDS provisioner listening", "address", httpServer.Addr)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("PDS provisioner failed", "error", err)
		os.Exit(1)
	}
}

func (s *server) health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte("{\"status\":\"ok\"}"))
}

func (s *server) invites(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	presented := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if len(presented) != len(s.token) ||
		subtle.ConstantTimeCompare([]byte(presented), []byte(s.token)) != 1 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	var input struct {
		UserID string `json:"user_id"`
		Handle string `json:"handle"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil ||
		strings.TrimSpace(input.UserID) == "" || strings.TrimSpace(input.Handle) == "" {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	code, err := s.createSingleUseInvite(r.Context())
	if err != nil {
		slog.Error("PDS invitation issuance failed", "error", err)
		http.Error(w, "PDS invitation issuance failed", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{"code": code})
}

func (s *server) createSingleUseInvite(ctx context.Context) (string, error) {
	body := bytes.NewBufferString("{\"useCount\":1}")
	endpoint := s.pdsURL + "/xrpc/com.atproto.server.createInviteCode"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return "", err
	}
	credential := base64.StdEncoding.EncodeToString([]byte("admin:" + s.adminPass))
	request.Header.Set("Authorization", "Basic "+credential)
	request.Header.Set("Content-Type", "application/json")
	response, err := s.client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("PDS returned HTTP %d", response.StatusCode)
	}
	var result struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return "", err
	}
	if strings.TrimSpace(result.Code) == "" {
		return "", errors.New("PDS returned empty invitation code")
	}
	return result.Code, nil
}
