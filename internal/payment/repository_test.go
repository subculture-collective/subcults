package payment

import (
	"testing"
)

// TestCreatePending_Success tests successful creation of a pending payment record.
func TestCreatePending_Success(t *testing.T) {
	repo := NewInMemoryPaymentRepository()

	record := &PaymentRecord{
		SessionID: "cs_test_123",
		Amount:    1000,
		Fee:       100,
		Currency:  "usd",
		UserDID:   "did:plc:user123",
		SceneID:   "scene-1",
	}

	err := repo.CreatePending(record)
	if err != nil {
		t.Fatalf("CreatePending failed: %v", err)
	}

	// Verify the record was created with pending status
	retrieved, err := repo.GetBySessionID("cs_test_123")
	if err != nil {
		t.Fatalf("GetBySessionID failed: %v", err)
	}

	if retrieved.Status != StatusPending {
		t.Errorf("expected status %s, got %s", StatusPending, retrieved.Status)
	}
	if retrieved.ID == "" {
		t.Error("expected ID to be set")
	}
	if retrieved.CreatedAt == nil {
		t.Error("expected CreatedAt to be set")
	}
	if retrieved.UpdatedAt == nil {
		t.Error("expected UpdatedAt to be set")
	}
}

// TestCreatePending_DuplicateSessionID tests that duplicate session IDs are rejected.
func TestCreatePending_DuplicateSessionID(t *testing.T) {
	repo := NewInMemoryPaymentRepository()

	record1 := &PaymentRecord{
		SessionID: "cs_test_123",
		Amount:    1000,
		Fee:       100,
		Currency:  "usd",
		UserDID:   "did:plc:user123",
		SceneID:   "scene-1",
	}

	err := repo.CreatePending(record1)
	if err != nil {
		t.Fatalf("first CreatePending failed: %v", err)
	}

	// Try to create another record with the same session ID
	record2 := &PaymentRecord{
		SessionID: "cs_test_123",
		Amount:    2000,
		Fee:       200,
		Currency:  "usd",
		UserDID:   "did:plc:user456",
		SceneID:   "scene-2",
	}

	err = repo.CreatePending(record2)
	if err != ErrDuplicateSessionID {
		t.Errorf("expected ErrDuplicateSessionID, got %v", err)
	}
}

// TestMarkCompleted_Success tests successful completion of a pending payment.
func TestMarkCompleted_Success(t *testing.T) {
	repo := NewInMemoryPaymentRepository()

	// Create a pending payment
	record := &PaymentRecord{
		SessionID: "cs_test_123",
		Amount:    1000,
		Fee:       100,
		Currency:  "usd",
		UserDID:   "did:plc:user123",
		SceneID:   "scene-1",
	}

	err := repo.CreatePending(record)
	if err != nil {
		t.Fatalf("CreatePending failed: %v", err)
	}

	// Mark as completed
	intentID := "pi_test_456"
	err = repo.MarkCompleted("cs_test_123", intentID)
	if err != nil {
		t.Fatalf("MarkCompleted failed: %v", err)
	}

	// Verify the status was updated
	retrieved, err := repo.GetBySessionID("cs_test_123")
	if err != nil {
		t.Fatalf("GetBySessionID failed: %v", err)
	}

	if retrieved.Status != StatusSucceeded {
		t.Errorf("expected status %s, got %s", StatusSucceeded, retrieved.Status)
	}
	if retrieved.PaymentIntentID == nil || *retrieved.PaymentIntentID != intentID {
		t.Errorf("expected PaymentIntentID %s, got %v", intentID, retrieved.PaymentIntentID)
	}
}

// TestMarkCompleted_Idempotent tests that marking completed is idempotent.
func TestMarkCompleted_Idempotent(t *testing.T) {
	repo := NewInMemoryPaymentRepository()

	record := &PaymentRecord{
		SessionID: "cs_test_123",
		Amount:    1000,
		Fee:       100,
		Currency:  "usd",
		UserDID:   "did:plc:user123",
		SceneID:   "scene-1",
	}

	err := repo.CreatePending(record)
	if err != nil {
		t.Fatalf("CreatePending failed: %v", err)
	}

	// Mark as completed
	intentID := "pi_test_456"
	err = repo.MarkCompleted("cs_test_123", intentID)
	if err != nil {
		t.Fatalf("first MarkCompleted failed: %v", err)
	}

	// Mark as completed again - should succeed (idempotent)
	err = repo.MarkCompleted("cs_test_123", intentID)
	if err != nil {
		t.Errorf("second MarkCompleted should be idempotent but got error: %v", err)
	}
}

// TestMarkCompleted_NotFound tests marking completed for non-existent session.
func TestMarkCompleted_NotFound(t *testing.T) {
	repo := NewInMemoryPaymentRepository()

	err := repo.MarkCompleted("cs_nonexistent", "pi_test_456")
	if err != ErrPaymentRecordNotFound {
		t.Errorf("expected ErrPaymentRecordNotFound, got %v", err)
	}
}

// TestMarkCompleted_InvalidTransition tests invalid status transitions.
func TestMarkCompleted_InvalidTransition(t *testing.T) {
	tests := []struct {
		name          string
		initialStatus string
		wantError     error
	}{
		{
			name:          "failed to succeeded - should reject",
			initialStatus: StatusFailed,
			wantError:     ErrInvalidStatusTransition,
		},
		{
			name:          "canceled to succeeded - should reject",
			initialStatus: StatusCanceled,
			wantError:     ErrInvalidStatusTransition,
		},
		{
			name:          "refunded to succeeded - should reject",
			initialStatus: StatusRefunded,
			wantError:     ErrInvalidStatusTransition,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewInMemoryPaymentRepository()

			// Create a record with the initial status
			record := &PaymentRecord{
				ID:        "payment-1",
				SessionID: "cs_test_123",
				Status:    tt.initialStatus,
				Amount:    1000,
				Fee:       100,
				Currency:  "usd",
				UserDID:   "did:plc:user123",
				SceneID:   "scene-1",
			}

			// Manually insert with the initial status (bypassing CreatePending)
			repo.records[record.ID] = record
			repo.sessions[record.SessionID] = record.ID

			// Try to mark as completed
			err := repo.MarkCompleted("cs_test_123", "pi_test_456")
			if err != tt.wantError {
				t.Errorf("expected error %v, got %v", tt.wantError, err)
			}
		})
	}
}

// TestMarkFailed_Success tests successful failure marking of a pending payment.
func TestMarkFailed_Success(t *testing.T) {
	repo := NewInMemoryPaymentRepository()

	// Create a pending payment
	record := &PaymentRecord{
		SessionID: "cs_test_123",
		Amount:    1000,
		Fee:       100,
		Currency:  "usd",
		UserDID:   "did:plc:user123",
		SceneID:   "scene-1",
	}

	err := repo.CreatePending(record)
	if err != nil {
		t.Fatalf("CreatePending failed: %v", err)
	}

	// Mark as failed
	reason := "card_declined"
	err = repo.MarkFailed("cs_test_123", reason)
	if err != nil {
		t.Fatalf("MarkFailed failed: %v", err)
	}

	// Verify the status was updated
	retrieved, err := repo.GetBySessionID("cs_test_123")
	if err != nil {
		t.Fatalf("GetBySessionID failed: %v", err)
	}

	if retrieved.Status != StatusFailed {
		t.Errorf("expected status %s, got %s", StatusFailed, retrieved.Status)
	}
	if retrieved.FailureReason == nil || *retrieved.FailureReason != reason {
		t.Errorf("expected FailureReason %s, got %v", reason, retrieved.FailureReason)
	}
}

// TestMarkFailed_Idempotent tests that marking failed is idempotent with same reason.
func TestMarkFailed_Idempotent(t *testing.T) {
	repo := NewInMemoryPaymentRepository()

	record := &PaymentRecord{
		SessionID: "cs_test_123",
		Amount:    1000,
		Fee:       100,
		Currency:  "usd",
		UserDID:   "did:plc:user123",
		SceneID:   "scene-1",
	}

	err := repo.CreatePending(record)
	if err != nil {
		t.Fatalf("CreatePending failed: %v", err)
	}

	// Mark as failed
	reason := "card_declined"
	err = repo.MarkFailed("cs_test_123", reason)
	if err != nil {
		t.Fatalf("first MarkFailed failed: %v", err)
	}

	// Mark as failed again with same reason - should succeed (idempotent)
	err = repo.MarkFailed("cs_test_123", reason)
	if err != nil {
		t.Errorf("second MarkFailed should be idempotent but got error: %v", err)
	}
}

// TestMarkFailed_UpdateReason tests that failure reason can be updated.
func TestMarkFailed_UpdateReason(t *testing.T) {
	repo := NewInMemoryPaymentRepository()

	record := &PaymentRecord{
		SessionID: "cs_test_123",
		Amount:    1000,
		Fee:       100,
		Currency:  "usd",
		UserDID:   "did:plc:user123",
		SceneID:   "scene-1",
	}

	err := repo.CreatePending(record)
	if err != nil {
		t.Fatalf("CreatePending failed: %v", err)
	}

	// Mark as failed with first reason
	reason1 := "card_declined"
	err = repo.MarkFailed("cs_test_123", reason1)
	if err != nil {
		t.Fatalf("first MarkFailed failed: %v", err)
	}

	// Mark as failed with different reason - should update the reason
	reason2 := "insufficient_funds"
	err = repo.MarkFailed("cs_test_123", reason2)
	if err != nil {
		t.Fatalf("second MarkFailed with different reason failed: %v", err)
	}

	// Verify the reason was updated
	retrieved, err := repo.GetBySessionID("cs_test_123")
	if err != nil {
		t.Fatalf("GetBySessionID failed: %v", err)
	}

	if retrieved.FailureReason == nil || *retrieved.FailureReason != reason2 {
		t.Errorf("expected FailureReason %s, got %v", reason2, retrieved.FailureReason)
	}
}

// TestMarkFailed_NotFound tests marking failed for non-existent session.
func TestMarkFailed_NotFound(t *testing.T) {
	repo := NewInMemoryPaymentRepository()

	err := repo.MarkFailed("cs_nonexistent", "card_declined")
	if err != ErrPaymentRecordNotFound {
		t.Errorf("expected ErrPaymentRecordNotFound, got %v", err)
	}
}

// TestMarkFailed_InvalidTransition tests invalid status transitions for failure.
func TestMarkFailed_InvalidTransition(t *testing.T) {
	tests := []struct {
		name          string
		initialStatus string
		wantError     error
	}{
		{
			name:          "succeeded to failed - should reject",
			initialStatus: StatusSucceeded,
			wantError:     ErrInvalidStatusTransition,
		},
		{
			name:          "canceled to failed - should reject",
			initialStatus: StatusCanceled,
			wantError:     ErrInvalidStatusTransition,
		},
		{
			name:          "refunded to failed - should reject",
			initialStatus: StatusRefunded,
			wantError:     ErrInvalidStatusTransition,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewInMemoryPaymentRepository()

			// Create a record with the initial status
			record := &PaymentRecord{
				ID:        "payment-1",
				SessionID: "cs_test_123",
				Status:    tt.initialStatus,
				Amount:    1000,
				Fee:       100,
				Currency:  "usd",
				UserDID:   "did:plc:user123",
				SceneID:   "scene-1",
			}

			// Manually insert with the initial status
			repo.records[record.ID] = record
			repo.sessions[record.SessionID] = record.ID

			// Try to mark as failed
			err := repo.MarkFailed("cs_test_123", "card_declined")
			if err != tt.wantError {
				t.Errorf("expected error %v, got %v", tt.wantError, err)
			}
		})
	}
}

// TestStatusTransitions_TableDriven tests all valid and invalid status transitions.
func TestStatusTransitions_TableDriven(t *testing.T) {
	tests := []struct {
		from      string
		to        string
		wantValid bool
	}{
		// Valid transitions from pending
		{StatusPending, StatusSucceeded, true},
		{StatusPending, StatusFailed, true},
		{StatusPending, StatusCanceled, true},
		
		// Valid transition from succeeded
		{StatusSucceeded, StatusRefunded, true},
		
		// Invalid transitions from pending
		{StatusPending, StatusRefunded, false},
		
		// Invalid transitions from succeeded
		{StatusSucceeded, StatusPending, false},
		{StatusSucceeded, StatusFailed, false},
		{StatusSucceeded, StatusCanceled, false},
		
		// Invalid transitions from failed
		{StatusFailed, StatusPending, false},
		{StatusFailed, StatusSucceeded, false},
		{StatusFailed, StatusCanceled, false},
		{StatusFailed, StatusRefunded, false},
		
		// Invalid transitions from canceled
		{StatusCanceled, StatusPending, false},
		{StatusCanceled, StatusSucceeded, false},
		{StatusCanceled, StatusFailed, false},
		{StatusCanceled, StatusRefunded, false},
		
		// Invalid transitions from refunded
		{StatusRefunded, StatusPending, false},
		{StatusRefunded, StatusSucceeded, false},
		{StatusRefunded, StatusFailed, false},
		{StatusRefunded, StatusCanceled, false},
	}

	for _, tt := range tests {
		t.Run(tt.from+"_to_"+tt.to, func(t *testing.T) {
			got := isValidStatusTransition(tt.from, tt.to)
			if got != tt.wantValid {
				t.Errorf("isValidStatusTransition(%s, %s) = %v, want %v",
					tt.from, tt.to, got, tt.wantValid)
			}
		})
	}
}

// TestInsert_DuplicateSessionID tests that Insert rejects duplicate session IDs.
func TestInsert_DuplicateSessionID(t *testing.T) {
	repo := NewInMemoryPaymentRepository()

	record1 := &PaymentRecord{
		SessionID: "cs_test_123",
		Status:    StatusPending,
		Amount:    1000,
		Fee:       100,
		Currency:  "usd",
		UserDID:   "did:plc:user123",
		SceneID:   "scene-1",
	}

	err := repo.Insert(record1)
	if err != nil {
		t.Fatalf("first Insert failed: %v", err)
	}

	// Try to insert another record with the same session ID
	record2 := &PaymentRecord{
		SessionID: "cs_test_123",
		Status:    StatusPending,
		Amount:    2000,
		Fee:       200,
		Currency:  "usd",
		UserDID:   "did:plc:user456",
		SceneID:   "scene-2",
	}

	err = repo.Insert(record2)
	if err != ErrDuplicateSessionID {
		t.Errorf("expected ErrDuplicateSessionID, got %v", err)
	}
}

// TestGetBySessionID_Success tests successful retrieval by session ID.
func TestGetBySessionID_Success(t *testing.T) {
	repo := NewInMemoryPaymentRepository()

	record := &PaymentRecord{
		SessionID: "cs_test_123",
		Amount:    1000,
		Fee:       100,
		Currency:  "usd",
		UserDID:   "did:plc:user123",
		SceneID:   "scene-1",
	}

	err := repo.CreatePending(record)
	if err != nil {
		t.Fatalf("CreatePending failed: %v", err)
	}

	retrieved, err := repo.GetBySessionID("cs_test_123")
	if err != nil {
		t.Fatalf("GetBySessionID failed: %v", err)
	}

	if retrieved.SessionID != "cs_test_123" {
		t.Errorf("expected SessionID cs_test_123, got %s", retrieved.SessionID)
	}
	if retrieved.Amount != 1000 {
		t.Errorf("expected Amount 1000, got %d", retrieved.Amount)
	}
}

// TestGetBySessionID_NotFound tests retrieval of non-existent session.
func TestGetBySessionID_NotFound(t *testing.T) {
	repo := NewInMemoryPaymentRepository()

	_, err := repo.GetBySessionID("cs_nonexistent")
	if err != ErrPaymentRecordNotFound {
		t.Errorf("expected ErrPaymentRecordNotFound, got %v", err)
	}
}
