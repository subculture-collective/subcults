// Package api provides HTTP API utilities including standardized error handling.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/onnwee/subcults/internal/middleware"
)

// Common error codes used throughout the API.
const (
	// ErrCodeValidation indicates input validation failure.
	ErrCodeValidation = "validation_error"

	// ErrCodeAuthFailed indicates authentication failure.
	ErrCodeAuthFailed = "auth_failed"

	// ErrCodeNotFound indicates the requested resource was not found.
	ErrCodeNotFound = "not_found"

	// ErrCodeRateLimited indicates rate limit exceeded.
	ErrCodeRateLimited = "rate_limited"

	// ErrCodeInternal indicates an internal server error.
	ErrCodeInternal = "internal_error"

	// ErrCodeForbidden indicates the request is forbidden.
	ErrCodeForbidden = "forbidden"

	// ErrCodeConflict indicates a conflict with the current state.
	ErrCodeConflict = "conflict"

	// ErrCodeBadRequest indicates a malformed request.
	ErrCodeBadRequest = "bad_request"

	// ErrCodeInvalidPalette indicates an invalid palette configuration.
	ErrCodeInvalidPalette = "invalid_palette"

	// ErrCodeSceneDeleted indicates the scene has been deleted.
	ErrCodeSceneDeleted = "scene_deleted"

	// ErrCodeInvalidSceneName indicates scene name validation failure.
	ErrCodeInvalidSceneName = "invalid_scene_name"

	// ErrCodeDuplicateSceneName indicates scene name already exists for owner.
	ErrCodeDuplicateSceneName = "duplicate_scene_name"

	// ErrCodeInvalidTimeRange indicates event start time is not before end time.
	ErrCodeInvalidTimeRange = "invalid_time_range"

	// ErrCodeMissingTarget indicates post must have at least one of scene_id or event_id.
	ErrCodeMissingTarget = "missing_target"

	// ErrCodeUnsupportedType indicates an unsupported content type for upload.
	ErrCodeUnsupportedType = "unsupported_type"

	// ErrCodeInvalidWeight indicates alliance weight must be between 0.0 and 1.0.
	ErrCodeInvalidWeight = "invalid_weight"

	// ErrCodeAllianceDeleted indicates the alliance has been deleted.
	ErrCodeAllianceDeleted = "alliance_deleted"

	// ErrCodeSelfAlliance indicates attempt to create alliance where from_scene_id == to_scene_id.
	ErrCodeSelfAlliance = "self_alliance"

	// ErrCodeSceneNotFound indicates the scene was not found.
	ErrCodeSceneNotFound = "scene_not_found"

	// ErrCodeUnauthorized is an alias of ErrCodeAuthFailed for 401-style auth errors.
	ErrCodeUnauthorized = ErrCodeAuthFailed

	// ErrCodePaymentNotFound indicates the payment record was not found.
	ErrCodePaymentNotFound = "payment_not_found"
)

// ErrorResponse represents the standard error response format.
// All API errors return JSON in this structure: {"error": {"code": "...", "message": "..."}}
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail contains the error code and human-readable message.
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// WriteError writes a standardized JSON error response.
// It writes the appropriate HTTP status code and returns a JSON error body.
//
// Format: {"error": {"code": "error_code", "message": "Error description"}}
//
// The error_code will be automatically logged by the logging middleware
// for all 4xx and 5xx responses if you call SetErrorCode on the context
// and pass the updated context to WriteError.
//
// Example:
//
//	ctx := middleware.SetErrorCode(r.Context(), api.ErrCodeNotFound)
//	WriteError(w, ctx, http.StatusNotFound, api.ErrCodeNotFound, "Scene not found")
//
// Or in a handler with middleware:
//
//	func handler(w http.ResponseWriter, r *http.Request) {
//	    ctx := middleware.SetErrorCode(r.Context(), api.ErrCodeNotFound)
//	    api.WriteError(w, ctx, http.StatusNotFound, api.ErrCodeNotFound, "Scene not found")
//	}
func WriteError(w http.ResponseWriter, ctx context.Context, status int, code, message string) {
	// Update the context in the response writer if supported (for logging middleware)
	middleware.UpdateResponseContext(w, ctx)

	// Create error response
	errResp := ErrorResponse{
		Error: ErrorDetail{
			Code:    code,
			Message: message,
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(errResp)
	if err != nil {
		// Fallback to plain text if JSON marshaling fails
		slog.ErrorContext(ctx, "failed to marshal error response", "error", err)
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Internal server error"))
		return
	}

	// Write response
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if _, err := w.Write(data); err != nil {
		slog.ErrorContext(ctx, "failed to write error response", "error", err)
	}
}

// WriteAPIError maps a domain error to its HTTP equivalent and writes the response.
// When the error is a registered domain sentinel, it uses the pre-configured status,
// code, and message. Unknown errors produce a generic 500 Internal Server Error.
//
// Handlers that need the mapped values directly (e.g. conditional logging) should
// use MapDomainError instead.
func WriteAPIError(w http.ResponseWriter, ctx context.Context, err error) {
	if status, code, message, ok := MapDomainError(err); ok {
		WriteError(w, ctx, status, code, message)
		return
	}
	ctx = middleware.SetErrorCode(ctx, ErrCodeInternal)
	WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "An internal error occurred")
}

// DomainErrorMapping associates a sentinel error with its HTTP representation.
type DomainErrorMapping struct {
	// Target is the sentinel error to match against using errors.Is.
	Target error
	// Status is the HTTP status code.
	Status int
	// Code is the machine-readable error code returned in the JSON body.
	Code string
	// Message is the human-readable description returned in the JSON body.
	Message string
}

var domainMappings []DomainErrorMapping

// RegisterDomainError adds a sentinel error to the centralized mapping table.
// Callers should register errors at init time or early in main().
func RegisterDomainError(target error, status int, code string, message string) {
	domainMappings = append(domainMappings, DomainErrorMapping{
		Target:  target,
		Status:  status,
		Code:    code,
		Message: message,
	})
}

// MapDomainError looks up a domain sentinel error in the registered mappings.
// It uses errors.Is to walk the error chain, so wrapped errors match their sentinels.
// Returns (status, code, message, true) if found, or zero values and false otherwise.
func MapDomainError(err error) (int, string, string, bool) {
	for i := range domainMappings {
		m := &domainMappings[i]
		if errors.Is(err, m.Target) {
			return m.Status, m.Code, m.Message, true
		}
	}
	return 0, "", "", false
}

// StatusCodeMapping returns the recommended HTTP status code for common error codes.
// This is a convenience function to map error codes to HTTP status codes.
func StatusCodeMapping(code string) int {
	switch code {
	case ErrCodeValidation:
		return http.StatusBadRequest
	case ErrCodeAuthFailed:
		return http.StatusUnauthorized
	case ErrCodeNotFound:
		return http.StatusNotFound
	case ErrCodeRateLimited:
		return http.StatusTooManyRequests
	case ErrCodeForbidden:
		return http.StatusForbidden
	case ErrCodeConflict:
		return http.StatusConflict
	case ErrCodeBadRequest:
		return http.StatusBadRequest
	case ErrCodeInternal:
		return http.StatusInternalServerError
	case ErrCodeSceneDeleted:
		return http.StatusGone
	case ErrCodeDuplicateSceneName:
		return http.StatusConflict
	case ErrCodeInvalidSceneName:
		return http.StatusUnprocessableEntity
	case ErrCodeInvalidTimeRange:
		return http.StatusUnprocessableEntity
	case ErrCodeMissingTarget:
		return http.StatusUnprocessableEntity
	case ErrCodeUnsupportedType:
		return http.StatusUnsupportedMediaType
	case ErrCodeInvalidWeight:
		return http.StatusUnprocessableEntity
	case ErrCodeAllianceDeleted:
		return http.StatusGone
	case ErrCodeSelfAlliance:
		return http.StatusUnprocessableEntity
	case ErrCodeSceneNotFound:
		return http.StatusNotFound
	case ErrCodePaymentNotFound:
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}
