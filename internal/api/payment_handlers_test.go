package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/onnwee/subcults/internal/middleware"
	"github.com/onnwee/subcults/internal/scene"
	"github.com/stripe/stripe-go/v81"
)

// mockStripeClient is a mock implementation of the payment.Client interface for testing.
type mockStripeClient struct {
	createAccountFunc          func() (*stripe.Account, error)
	createAccountLinkFunc      func(accountID, returnURL, refreshURL string) (*stripe.AccountLink, error)
	createCheckoutSessionFunc  func(params *payment.CheckoutSessionParams) (*stripe.CheckoutSession, error)
}

func (m *mockStripeClient) CreateConnectAccount() (*stripe.Account, error) {
	if m.createAccountFunc != nil {
		return m.createAccountFunc()
	}
	return &stripe.Account{ID: "acct_test123"}, nil
}

func (m *mockStripeClient) CreateAccountLink(accountID, returnURL, refreshURL string) (*stripe.AccountLink, error) {
	if m.createAccountLinkFunc != nil {
		return m.createAccountLinkFunc(accountID, returnURL, refreshURL)
	}
	return &stripe.AccountLink{
		URL: "https://connect.stripe.com/setup/s/test123",
	}, nil
}

func (m *mockStripeClient) CreateCheckoutSession(params *payment.CheckoutSessionParams) (*stripe.CheckoutSession, error) {
	if m.createCheckoutSessionFunc != nil {
		return m.createCheckoutSessionFunc(params)
	}
	return &stripe.CheckoutSession{
		ID:  "cs_test123",
		URL: "https://checkout.stripe.com/pay/cs_test123",
	}, nil
}

// TestOnboardScene_Success tests successful scene onboarding.
func TestOnboardScene_Success(t *testing.T) {
	sceneRepo := scene.NewInMemorySceneRepository()
	paymentRepo := payment.NewInMemoryPaymentRepository()
	mockClient := &mockStripeClient{}
	handlers := NewPaymentHandlers(
		sceneRepo,
		paymentRepo,
		mockClient,
		"https://example.com/return",
		"https://example.com/refresh",
		5.0,
	)

	// Create a test scene
	testScene := &scene.Scene{
		ID:            "scene-1",
		Name:          "Test Scene",
		OwnerDID:      "did:plc:owner123",
		CoarseGeohash: "dr5regw",
	}
	if err := sceneRepo.Insert(testScene); err != nil {
		t.Fatalf("failed to create test scene: %v", err)
	}

	reqBody := OnboardSceneRequest{
		SceneID: "scene-1",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/payments/onboard", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := middleware.SetUserDID(req.Context(), "did:plc:owner123")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handlers.OnboardScene(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response OnboardSceneResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.URL == "" {
		t.Error("expected URL to be set")
	}
	if response.ExpiresAt == "" {
		t.Error("expected ExpiresAt to be set")
	}
	if response.URL != "https://connect.stripe.com/setup/s/test123" {
		t.Errorf("expected URL to be https://connect.stripe.com/setup/s/test123, got %s", response.URL)
	}

	// Verify scene was updated with connected account ID
	updatedScene, err := sceneRepo.GetByID("scene-1")
	if err != nil {
		t.Fatalf("failed to get updated scene: %v", err)
	}
	if updatedScene.ConnectedAccountID == nil || *updatedScene.ConnectedAccountID != "acct_test123" {
		t.Errorf("expected ConnectedAccountID to be acct_test123, got %v", updatedScene.ConnectedAccountID)
	}
}

// TestOnboardScene_Unauthorized tests onboarding without authentication.
func TestOnboardScene_Unauthorized(t *testing.T) {
	sceneRepo := scene.NewInMemorySceneRepository()
	paymentRepo := payment.NewInMemoryPaymentRepository()
	mockClient := &mockStripeClient{}
	handlers := NewPaymentHandlers(
		sceneRepo,
		paymentRepo,
		mockClient,
		"https://example.com/return",
		"https://example.com/refresh",
		5.0,
	)

	reqBody := OnboardSceneRequest{
		SceneID: "scene-1",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/payments/onboard", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// No user DID in context

	w := httptest.NewRecorder()
	handlers.OnboardScene(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

// TestOnboardScene_NotOwner tests onboarding by non-owner.
func TestOnboardScene_NotOwner(t *testing.T) {
	sceneRepo := scene.NewInMemorySceneRepository()
	paymentRepo := payment.NewInMemoryPaymentRepository()
	mockClient := &mockStripeClient{}
	handlers := NewPaymentHandlers(
		sceneRepo,
		paymentRepo,
		mockClient,
		"https://example.com/return",
		"https://example.com/refresh",
		5.0,
	)

	// Create a test scene owned by someone else
	testScene := &scene.Scene{
		ID:            "scene-1",
		Name:          "Test Scene",
		OwnerDID:      "did:plc:owner123",
		CoarseGeohash: "dr5regw",
	}
	if err := sceneRepo.Insert(testScene); err != nil {
		t.Fatalf("failed to create test scene: %v", err)
	}

	reqBody := OnboardSceneRequest{
		SceneID: "scene-1",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/payments/onboard", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := middleware.SetUserDID(req.Context(), "did:plc:notowner456")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handlers.OnboardScene(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", w.Code)
	}
}

// TestOnboardScene_AlreadyOnboarded tests duplicate onboarding attempt.
func TestOnboardScene_AlreadyOnboarded(t *testing.T) {
	sceneRepo := scene.NewInMemorySceneRepository()
	paymentRepo := payment.NewInMemoryPaymentRepository()
	mockClient := &mockStripeClient{}
	handlers := NewPaymentHandlers(
		sceneRepo,
		paymentRepo,
		mockClient,
		"https://example.com/return",
		"https://example.com/refresh",
		5.0,
	)

	// Create a test scene with existing connected account
	existingAccountID := "acct_existing123"
	testScene := &scene.Scene{
		ID:                 "scene-1",
		Name:               "Test Scene",
		OwnerDID:           "did:plc:owner123",
		CoarseGeohash:      "dr5regw",
		ConnectedAccountID: &existingAccountID,
	}
	if err := sceneRepo.Insert(testScene); err != nil {
		t.Fatalf("failed to create test scene: %v", err)
	}

	reqBody := OnboardSceneRequest{
		SceneID: "scene-1",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/payments/onboard", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := middleware.SetUserDID(req.Context(), "did:plc:owner123")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handlers.OnboardScene(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	// Verify error code is "already_onboarded"
	var errorResp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&errorResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errorResp.Error.Code != "already_onboarded" {
		t.Errorf("expected error code 'already_onboarded', got %v", errorResp.Error.Code)
	}
}

// TestOnboardScene_SceneNotFound tests onboarding for non-existent scene.
func TestOnboardScene_SceneNotFound(t *testing.T) {
	sceneRepo := scene.NewInMemorySceneRepository()
	paymentRepo := payment.NewInMemoryPaymentRepository()
	mockClient := &mockStripeClient{}
	handlers := NewPaymentHandlers(
		sceneRepo,
		paymentRepo,
		mockClient,
		"https://example.com/return",
		"https://example.com/refresh",
		5.0,
	)

	reqBody := OnboardSceneRequest{
		SceneID: "non-existent-scene",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/payments/onboard", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := middleware.SetUserDID(req.Context(), "did:plc:owner123")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handlers.OnboardScene(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// TestOnboardScene_MissingSceneID tests onboarding without scene ID.
func TestOnboardScene_MissingSceneID(t *testing.T) {
	sceneRepo := scene.NewInMemorySceneRepository()
	paymentRepo := payment.NewInMemoryPaymentRepository()
	mockClient := &mockStripeClient{}
	handlers := NewPaymentHandlers(
		sceneRepo,
		paymentRepo,
		mockClient,
		"https://example.com/return",
		"https://example.com/refresh",
		5.0,
	)

	reqBody := OnboardSceneRequest{
		SceneID: "",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/payments/onboard", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := middleware.SetUserDID(req.Context(), "did:plc:owner123")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handlers.OnboardScene(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// TestOnboardScene_StripeAccountCreationFails tests Stripe account creation failure.
func TestOnboardScene_StripeAccountCreationFails(t *testing.T) {
	sceneRepo := scene.NewInMemorySceneRepository()
	paymentRepo := payment.NewInMemoryPaymentRepository()
	mockClient := &mockStripeClient{
		createAccountFunc: func() (*stripe.Account, error) {
			return nil, errors.New("stripe account creation failed")
		},
	}
	handlers := NewPaymentHandlers(
		sceneRepo,
		paymentRepo,
		mockClient,
		"https://example.com/return",
		"https://example.com/refresh",
		5.0,
	)

	// Create a test scene
	testScene := &scene.Scene{
		ID:            "scene-1",
		Name:          "Test Scene",
		OwnerDID:      "did:plc:owner123",
		CoarseGeohash: "dr5regw",
	}
	if err := sceneRepo.Insert(testScene); err != nil {
		t.Fatalf("failed to create test scene: %v", err)
	}

	reqBody := OnboardSceneRequest{
		SceneID: "scene-1",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/payments/onboard", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := middleware.SetUserDID(req.Context(), "did:plc:owner123")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handlers.OnboardScene(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

// TestOnboardScene_StripeLinkCreationFails tests Stripe link creation failure.
func TestOnboardScene_StripeLinkCreationFails(t *testing.T) {
	sceneRepo := scene.NewInMemorySceneRepository()
	paymentRepo := payment.NewInMemoryPaymentRepository()
	mockClient := &mockStripeClient{
		createAccountLinkFunc: func(accountID, returnURL, refreshURL string) (*stripe.AccountLink, error) {
			return nil, errors.New("stripe link creation failed")
		},
	}
	handlers := NewPaymentHandlers(
		sceneRepo,
		paymentRepo,
		mockClient,
		"https://example.com/return",
		"https://example.com/refresh",
		5.0,
	)

	// Create a test scene
	testScene := &scene.Scene{
		ID:            "scene-1",
		Name:          "Test Scene",
		OwnerDID:      "did:plc:owner123",
		CoarseGeohash: "dr5regw",
	}
	if err := sceneRepo.Insert(testScene); err != nil {
		t.Fatalf("failed to create test scene: %v", err)
	}

	reqBody := OnboardSceneRequest{
		SceneID: "scene-1",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/payments/onboard", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := middleware.SetUserDID(req.Context(), "did:plc:owner123")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handlers.OnboardScene(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

// TestCreateCheckoutSession_Success tests successful checkout session creation.
func TestCreateCheckoutSession_Success(t *testing.T) {
sceneRepo := scene.NewInMemorySceneRepository()
paymentRepo := payment.NewInMemoryPaymentRepository()
mockClient := &mockStripeClient{}
handlers := NewPaymentHandlers(
sceneRepo,
paymentRepo,
mockClient,
"https://example.com/return",
"https://example.com/refresh",
5.0,
)

// Create a test scene with connected account
connectedAccountID := "acct_test123"
testScene := &scene.Scene{
ID:                 "scene-1",
Name:               "Test Scene",
OwnerDID:           "did:plc:owner123",
CoarseGeohash:      "dr5regw",
ConnectedAccountID: &connectedAccountID,
}
if err := sceneRepo.Insert(testScene); err != nil {
t.Fatalf("failed to create test scene: %v", err)
}

reqBody := CheckoutSessionRequest{
SceneID: "scene-1",
Items: []CheckoutItemRequest{
{PriceID: "price_test123", Quantity: 2},
},
SuccessURL: "https://example.com/success",
CancelURL:  "https://example.com/cancel",
}
body, _ := json.Marshal(reqBody)

req := httptest.NewRequest(http.MethodPost, "/payments/checkout", bytes.NewReader(body))
req.Header.Set("Content-Type", "application/json")
ctx := middleware.SetUserDID(req.Context(), "did:plc:user123")
req = req.WithContext(ctx)

w := httptest.NewRecorder()
handlers.CreateCheckoutSession(w, req)

if w.Code != http.StatusOK {
t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
}

var response CheckoutSessionResponse
if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
t.Fatalf("failed to decode response: %v", err)
}

if response.SessionURL == "" {
t.Error("expected SessionURL to be set")
}
if response.SessionID == "" {
t.Error("expected SessionID to be set")
}

// Verify payment record was created
record, err := paymentRepo.GetBySessionID(response.SessionID)
if err != nil {
t.Errorf("expected payment record to exist, got error: %v", err)
}
if record.Status != payment.StatusPending {
t.Errorf("expected status pending, got %s", record.Status)
}
if record.SceneID != "scene-1" {
t.Errorf("expected scene_id scene-1, got %s", record.SceneID)
}
if record.UserDID != "did:plc:user123" {
t.Errorf("expected user_did did:plc:user123, got %s", record.UserDID)
}
}

// TestCreateCheckoutSession_Unauthorized tests checkout session creation without authentication.
func TestCreateCheckoutSession_Unauthorized(t *testing.T) {
sceneRepo := scene.NewInMemorySceneRepository()
paymentRepo := payment.NewInMemoryPaymentRepository()
mockClient := &mockStripeClient{}
handlers := NewPaymentHandlers(
sceneRepo,
paymentRepo,
mockClient,
"https://example.com/return",
"https://example.com/refresh",
5.0,
)

reqBody := CheckoutSessionRequest{
SceneID: "scene-1",
Items: []CheckoutItemRequest{
{PriceID: "price_test123", Quantity: 1},
},
SuccessURL: "https://example.com/success",
CancelURL:  "https://example.com/cancel",
}
body, _ := json.Marshal(reqBody)

req := httptest.NewRequest(http.MethodPost, "/payments/checkout", bytes.NewReader(body))
req.Header.Set("Content-Type", "application/json")
// No user DID in context

w := httptest.NewRecorder()
handlers.CreateCheckoutSession(w, req)

if w.Code != http.StatusUnauthorized {
t.Errorf("expected status 401, got %d", w.Code)
}
}

// TestCreateCheckoutSession_SceneNotOnboarded tests checkout for scene without connected account.
func TestCreateCheckoutSession_SceneNotOnboarded(t *testing.T) {
sceneRepo := scene.NewInMemorySceneRepository()
paymentRepo := payment.NewInMemoryPaymentRepository()
mockClient := &mockStripeClient{}
handlers := NewPaymentHandlers(
sceneRepo,
paymentRepo,
mockClient,
"https://example.com/return",
"https://example.com/refresh",
5.0,
)

// Create a test scene without connected account
testScene := &scene.Scene{
ID:            "scene-1",
Name:          "Test Scene",
OwnerDID:      "did:plc:owner123",
CoarseGeohash: "dr5regw",
}
if err := sceneRepo.Insert(testScene); err != nil {
t.Fatalf("failed to create test scene: %v", err)
}

reqBody := CheckoutSessionRequest{
SceneID: "scene-1",
Items: []CheckoutItemRequest{
{PriceID: "price_test123", Quantity: 1},
},
SuccessURL: "https://example.com/success",
CancelURL:  "https://example.com/cancel",
}
body, _ := json.Marshal(reqBody)

req := httptest.NewRequest(http.MethodPost, "/payments/checkout", bytes.NewReader(body))
req.Header.Set("Content-Type", "application/json")
ctx := middleware.SetUserDID(req.Context(), "did:plc:user123")
req = req.WithContext(ctx)

w := httptest.NewRecorder()
handlers.CreateCheckoutSession(w, req)

if w.Code != http.StatusBadRequest {
t.Errorf("expected status 400, got %d", w.Code)
}

var errorResp ErrorResponse
if err := json.NewDecoder(w.Body).Decode(&errorResp); err != nil {
t.Fatalf("failed to decode error response: %v", err)
}
if errorResp.Error.Code != "not_onboarded" {
t.Errorf("expected error code 'not_onboarded', got %v", errorResp.Error.Code)
}
}

// TestCreateCheckoutSession_InvalidPriceID tests checkout with invalid price ID via Stripe error.
func TestCreateCheckoutSession_InvalidPriceID(t *testing.T) {
sceneRepo := scene.NewInMemorySceneRepository()
paymentRepo := payment.NewInMemoryPaymentRepository()
mockClient := &mockStripeClient{
createCheckoutSessionFunc: func(params *payment.CheckoutSessionParams) (*stripe.CheckoutSession, error) {
return nil, errors.New("invalid price_id")
},
}
handlers := NewPaymentHandlers(
sceneRepo,
paymentRepo,
mockClient,
"https://example.com/return",
"https://example.com/refresh",
5.0,
)

// Create a test scene with connected account
connectedAccountID := "acct_test123"
testScene := &scene.Scene{
ID:                 "scene-1",
Name:               "Test Scene",
OwnerDID:           "did:plc:owner123",
CoarseGeohash:      "dr5regw",
ConnectedAccountID: &connectedAccountID,
}
if err := sceneRepo.Insert(testScene); err != nil {
t.Fatalf("failed to create test scene: %v", err)
}

reqBody := CheckoutSessionRequest{
SceneID: "scene-1",
Items: []CheckoutItemRequest{
{PriceID: "invalid_price", Quantity: 1},
},
SuccessURL: "https://example.com/success",
CancelURL:  "https://example.com/cancel",
}
body, _ := json.Marshal(reqBody)

req := httptest.NewRequest(http.MethodPost, "/payments/checkout", bytes.NewReader(body))
req.Header.Set("Content-Type", "application/json")
ctx := middleware.SetUserDID(req.Context(), "did:plc:user123")
req = req.WithContext(ctx)

w := httptest.NewRecorder()
handlers.CreateCheckoutSession(w, req)

if w.Code != http.StatusInternalServerError {
t.Errorf("expected status 500, got %d", w.Code)
}
}

// TestCreateCheckoutSession_EmptyItems tests checkout without items.
func TestCreateCheckoutSession_EmptyItems(t *testing.T) {
sceneRepo := scene.NewInMemorySceneRepository()
paymentRepo := payment.NewInMemoryPaymentRepository()
mockClient := &mockStripeClient{}
handlers := NewPaymentHandlers(
sceneRepo,
paymentRepo,
mockClient,
"https://example.com/return",
"https://example.com/refresh",
5.0,
)

reqBody := CheckoutSessionRequest{
SceneID:    "scene-1",
Items:      []CheckoutItemRequest{},
SuccessURL: "https://example.com/success",
CancelURL:  "https://example.com/cancel",
}
body, _ := json.Marshal(reqBody)

req := httptest.NewRequest(http.MethodPost, "/payments/checkout", bytes.NewReader(body))
req.Header.Set("Content-Type", "application/json")
ctx := middleware.SetUserDID(req.Context(), "did:plc:user123")
req = req.WithContext(ctx)

w := httptest.NewRecorder()
handlers.CreateCheckoutSession(w, req)

if w.Code != http.StatusBadRequest {
t.Errorf("expected status 400, got %d", w.Code)
}
}
