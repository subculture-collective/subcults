package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/onnwee/subcults/internal/audience"
	"github.com/onnwee/subcults/internal/middleware"
	"github.com/onnwee/subcults/internal/signal"
)

func TestSignalHandlersCreateDraftRequiresAuth(t *testing.T) {
	h := NewSignalHandlers(signal.NewService(signal.NewInMemoryRepository()))
	req := httptest.NewRequest(http.MethodPost, "/signals", bytes.NewBufferString(`{}`))
	rr := httptest.NewRecorder()
	h.CreateDraft(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rr.Code)
	}
}
func TestSignalHandlersCreateDraft(t *testing.T) {
	h := NewSignalHandlers(signal.NewService(signal.NewInMemoryRepository()))
	req := httptest.NewRequest(http.MethodPost, "/signals", bytes.NewBufferString(`{"id":"s1","owner_type":"profile","owner_id":"p1","target_type":"tour","target_id":"t1","body":"Tickets live","audience_definition":{"segment":"all"}}`))
	req = req.WithContext(middleware.SetUserDID(req.Context(), "did:owner"))
	rr := httptest.NewRecorder()
	h.CreateDraft(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestSignalHandlersHideDraftAndReturnPublicPublishedDTO(t *testing.T) {
	repository := signal.NewInMemoryRepository()
	service := signal.NewService(repository)
	h := NewSignalHandlers(service)
	if _, err := service.CreateDraft(context.Background(), signal.Signal{ID: "s1", OwnerType: "profile", OwnerID: "p1", TargetType: "tour", TargetID: "t1"}, signal.Content{Subject: "Tickets", Body: "On sale"}, `{"segment":"all"}`, "did:owner", nil); err != nil {
		t.Fatal(err)
	}

	draftResponse := httptest.NewRecorder()
	h.Get(draftResponse, httptest.NewRequest(http.MethodGet, "/api/signals/s1", nil))
	if draftResponse.Code != http.StatusNotFound {
		t.Fatalf("draft status=%d body=%s", draftResponse.Code, draftResponse.Body.String())
	}

	if _, err := service.Publish(context.Background(), "s1"); err != nil {
		t.Fatal(err)
	}
	publishedResponse := httptest.NewRecorder()
	h.Get(publishedResponse, httptest.NewRequest(http.MethodGet, "/api/signals/s1", nil))
	if publishedResponse.Code != http.StatusOK {
		t.Fatalf("published status=%d body=%s", publishedResponse.Code, publishedResponse.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(publishedResponse.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	publicSignal, ok := body["signal"].(map[string]any)
	if !ok || publicSignal["title"] != "Tickets" || publicSignal["ID"] != nil {
		t.Fatalf("public body=%v", body)
	}
}

func TestSignalHandlersPublishRequiresAuth(t *testing.T) {
	h := NewSignalHandlers(signal.NewService(signal.NewInMemoryRepository()))
	response := httptest.NewRecorder()
	h.Publish(response, httptest.NewRequest(http.MethodPost, "/signals/s1/publish", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestSignalHandlersConsentUsesAuthenticatedContactLink(t *testing.T) {
	ctx := context.Background()
	audienceService := audience.NewService(audience.NewInMemoryRepository())
	verifiedAt := time.Now().UTC()
	if err := audienceService.AddContact(ctx, audience.ContactPoint{ID: "contact-1", Kind: "email", VerifiedAt: &verifiedAt}); err != nil {
		t.Fatal(err)
	}
	if err := audienceService.LinkContact(ctx, audience.ContactPointLink{ContactPointID: "contact-1", UserDID: "did:fan", VerifiedAt: verifiedAt}); err != nil {
		t.Fatal(err)
	}
	scopeID, err := audienceService.CreateScope(ctx, audience.DeliveryScope{SenderType: "profile", SenderID: "p1", Channel: "email", Purpose: "tour_updates", DisclosureVersion: "2026-08"})
	if err != nil {
		t.Fatal(err)
	}
	h := NewSignalHandlers(signal.NewService(signal.NewInMemoryRepository()), audienceService)
	request := httptest.NewRequest(http.MethodPost, "/api/audience/consent", bytes.NewBufferString(`{"scope_id":`+mustJSON(t, scopeID)+`,"action":"grant","contact_id":"attacker-choice"}`))
	request = request.WithContext(middleware.SetUserDID(request.Context(), "did:fan"))
	response := httptest.NewRecorder()
	h.MutateConsent(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Consent publicConsentState `json:"consent"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Consent.Status != "granted" || body.Consent.Scope.ID != scopeID {
		t.Fatalf("consent=%+v", body.Consent)
	}
}

func mustJSON(t *testing.T, value string) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}
