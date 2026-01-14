// Package api provides HTTP handlers for the Subcults API.
package api

import (
	"encoding/json"
	"html"
	"log/slog"
	"net/http"
	"strings"

	"github.com/onnwee/subcults/internal/middleware"
	"github.com/onnwee/subcults/internal/post"
)

// Post text validation constraints
const (
	MaxPostTextLength = 5000
	MaxAttachments    = 6
)

// CreatePostRequest represents the request body for creating a post.
type CreatePostRequest struct {
	SceneID     *string            `json:"scene_id,omitempty"`
	EventID     *string            `json:"event_id,omitempty"`
	Text        string             `json:"text"`
	Attachments []post.Attachment  `json:"attachments,omitempty"`
	Labels      []string           `json:"labels,omitempty"`
}

// UpdatePostRequest represents the request body for updating a post.
type UpdatePostRequest struct {
	Text        *string            `json:"text,omitempty"`
	Attachments *[]post.Attachment `json:"attachments,omitempty"`
	Labels      *[]string          `json:"labels,omitempty"`
}

// PostHandlers holds dependencies for post HTTP handlers.
type PostHandlers struct {
	repo post.PostRepository
}

// NewPostHandlers creates a new PostHandlers instance.
func NewPostHandlers(repo post.PostRepository) *PostHandlers {
	return &PostHandlers{
		repo: repo,
	}
}

// validatePostText validates post text according to requirements.
// Returns error message if validation fails, empty string if valid.
func validatePostText(text string) string {
	// Trim whitespace first
	trimmed := strings.TrimSpace(text)
	
	if trimmed == "" {
		return "post text is required"
	}
	if len(trimmed) > MaxPostTextLength {
		return "post text must not exceed 5000 characters"
	}
	return ""
}

// sanitizePostText sanitizes post text to prevent XSS attacks.
// Strips HTML tags by escaping HTML entities.
// Should be called after validation passes.
func sanitizePostText(text string) string {
	return html.EscapeString(strings.TrimSpace(text))
}

// CreatePost handles POST /posts - creates a new post.
func (h *PostHandlers) CreatePost(w http.ResponseWriter, r *http.Request) {
	var req CreatePostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Invalid JSON in request body")
		return
	}

	// Validate at least one of sceneId/eventId is provided
	if req.SceneID == nil && req.EventID == nil {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeMissingTarget)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeMissingTarget, "Either scene_id or event_id must be provided")
		return
	}

	// Validate text
	if errMsg := validatePostText(req.Text); errMsg != "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, errMsg)
		return
	}

	// Sanitize text to prevent XSS
	req.Text = sanitizePostText(req.Text)

	// Validate attachments count
	if len(req.Attachments) > MaxAttachments {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, "Maximum 6 attachments allowed")
		return
	}

	// Sanitize labels
	sanitizedLabels := make([]string, len(req.Labels))
	for i, label := range req.Labels {
		sanitizedLabels[i] = html.EscapeString(strings.TrimSpace(label))
	}

	// Get author DID from context (would typically come from auth middleware)
	authorDID := middleware.GetUserDID(r.Context())
	if authorDID == "" {
		// For now, allow unauthenticated posts with a default DID
		// In production, this should return 401 Unauthorized
		authorDID = "did:example:anonymous"
	}

	// Create post
	newPost := &post.Post{
		SceneID:     req.SceneID,
		EventID:     req.EventID,
		AuthorDID:   authorDID,
		Text:        req.Text,
		Attachments: req.Attachments,
		Labels:      sanitizedLabels,
	}

	if err := h.repo.Create(newPost); err != nil {
		slog.ErrorContext(r.Context(), "failed to create post", "error", err)
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to create post")
		return
	}

	// Return created post
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(newPost); err != nil {
		slog.ErrorContext(r.Context(), "failed to encode response", "error", err)
		return
	}
}

// UpdatePost handles PATCH /posts/{id} - updates an existing post.
func (h *PostHandlers) UpdatePost(w http.ResponseWriter, r *http.Request) {
	// Extract post ID from URL path
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/posts/"), "/")
	if len(pathParts) == 0 || pathParts[0] == "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Post ID is required")
		return
	}
	postID := pathParts[0]

	// Parse request body
	var req UpdatePostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Invalid JSON in request body")
		return
	}

	// Get existing post
	existingPost, err := h.repo.GetByID(postID)
	if err != nil {
		if err == post.ErrPostNotFound {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeNotFound)
			WriteError(w, ctx, http.StatusNotFound, ErrCodeNotFound, "Post not found")
			return
		}
		slog.ErrorContext(r.Context(), "failed to retrieve post", "error", err, "post_id", postID)
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to retrieve post")
		return
	}

	// Apply updates
	if req.Text != nil {
		newText := *req.Text
		if errMsg := validatePostText(newText); errMsg != "" {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
			WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, errMsg)
			return
		}
		existingPost.Text = sanitizePostText(newText)
	}

	if req.Attachments != nil {
		if len(*req.Attachments) > MaxAttachments {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
			WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, "Maximum 6 attachments allowed")
			return
		}
		existingPost.Attachments = *req.Attachments
	}

	if req.Labels != nil {
		sanitizedLabels := make([]string, len(*req.Labels))
		for i, label := range *req.Labels {
			sanitizedLabels[i] = html.EscapeString(strings.TrimSpace(label))
		}
		existingPost.Labels = sanitizedLabels
	}

	// Update in repository
	if err := h.repo.Update(existingPost); err != nil {
		if err == post.ErrPostDeleted {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeNotFound)
			WriteError(w, ctx, http.StatusNotFound, ErrCodeNotFound, "Post not found")
			return
		}
		slog.ErrorContext(r.Context(), "failed to update post", "error", err, "post_id", postID)
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to update post")
		return
	}

	// Retrieve updated post
	updated, err := h.repo.GetByID(postID)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to retrieve updated post", "error", err, "post_id", postID)
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to retrieve updated post")
		return
	}

	// Return updated post
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(updated); err != nil {
		slog.ErrorContext(r.Context(), "failed to encode response", "error", err)
		return
	}
}

// DeletePost handles DELETE /posts/{id} - soft-deletes a post.
func (h *PostHandlers) DeletePost(w http.ResponseWriter, r *http.Request) {
	// Extract post ID from URL path
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/posts/"), "/")
	if len(pathParts) == 0 || pathParts[0] == "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Post ID is required")
		return
	}
	postID := pathParts[0]

	// Soft delete the post
	if err := h.repo.Delete(postID); err != nil {
		if err == post.ErrPostNotFound {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeNotFound)
			WriteError(w, ctx, http.StatusNotFound, ErrCodeNotFound, "Post not found")
			return
		}
		if err == post.ErrPostDeleted {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeNotFound)
			WriteError(w, ctx, http.StatusNotFound, ErrCodeNotFound, "Post not found")
			return
		}
		slog.ErrorContext(r.Context(), "failed to delete post", "error", err, "post_id", postID)
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to delete post")
		return
	}

	// Return success with no content
	w.WriteHeader(http.StatusNoContent)
}
