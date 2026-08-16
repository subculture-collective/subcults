// Package api provides HTTP handlers for the Subcults API.
package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/onnwee/subcults/internal/attachment"
	"github.com/onnwee/subcults/internal/membership"
	"github.com/onnwee/subcults/internal/middleware"
	"github.com/onnwee/subcults/internal/post"
	"github.com/onnwee/subcults/internal/scene"
)

// PostHandlers holds dependencies for post HTTP handlers.
type PostHandlers struct {
	postService     *post.Service
	sceneRepo       scene.SceneRepository
	membershipRepo  membership.MembershipRepository
	metadataService *attachment.MetadataService // Optional: for enriching attachment metadata
}

// NewPostHandlers creates a new PostHandlers instance.
// metadataService is optional and can be nil if attachment enrichment is not configured.
func NewPostHandlers(postService *post.Service, sceneRepo scene.SceneRepository, membershipRepo membership.MembershipRepository, metadataService *attachment.MetadataService) *PostHandlers {
	return &PostHandlers{
		postService:     postService,
		sceneRepo:       sceneRepo,
		membershipRepo:  membershipRepo,
		metadataService: metadataService,
	}
}

// CreatePostRequest represents the request body for creating a post.
type CreatePostRequest struct {
	SceneID     *string           `json:"scene_id,omitempty"`
	EventID     *string           `json:"event_id,omitempty"`
	Text        string            `json:"text"`
	Attachments []post.Attachment `json:"attachments,omitempty"`
	Labels      []string          `json:"labels,omitempty"`
}

// UpdatePostRequest represents the request body for updating a post.
type UpdatePostRequest struct {
	Text        *string            `json:"text,omitempty"`
	Attachments *[]post.Attachment `json:"attachments,omitempty"`
	Labels      *[]string          `json:"labels,omitempty"`
}

// extractPostID extracts the post ID from the URL path.
func extractPostID(r *http.Request) (string, error) {
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/posts/"), "/")
	if len(pathParts) == 0 || pathParts[0] == "" {
		return "", fmt.Errorf("post ID is required")
	}
	return pathParts[0], nil
}

// CreatePost handles POST /posts - creates a new post.
func (h *PostHandlers) CreatePost(w http.ResponseWriter, r *http.Request) {
	var req CreatePostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Invalid JSON in request body")
		return
	}

	// Get author DID from context (set by auth middleware)
	authorDID := middleware.GetUserDID(r.Context())
	if authorDID == "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeAuthFailed)
		WriteError(w, ctx, http.StatusUnauthorized, ErrCodeAuthFailed, "authentication required")
		return
	}

	// Enrich attachments with metadata if service is configured
	enrichedAttachments := make([]post.Attachment, 0, len(req.Attachments))
	if h.metadataService != nil {
		for _, att := range req.Attachments {
			if att.Key == "" {
				enrichedAttachments = append(enrichedAttachments, att)
				continue
			}
			enriched, err := h.metadataService.EnrichAttachment(r.Context(), att.Key)
			if err != nil {
				slog.WarnContext(r.Context(), "failed to enrich attachment",
					"key", att.Key,
					"error", err)
				enrichedAttachments = append(enrichedAttachments, att)
				continue
			}
			enrichedAttachments = append(enrichedAttachments, *enriched)
		}
	} else {
		enrichedAttachments = req.Attachments
	}

	// Delegate to service for validation and creation
	newPost, err := h.postService.CreatePost(post.CreatePostInput{
		Text:        req.Text,
		SceneID:     req.SceneID,
		EventID:     req.EventID,
		Attachments: enrichedAttachments,
		Labels:      req.Labels,
		AuthorDID:   authorDID,
	})
	if err != nil {
		code, status := mapError(err)
		ctx := middleware.SetErrorCode(r.Context(), code)
		WriteError(w, ctx, status, code, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(newPost); err != nil {
		slog.ErrorContext(r.Context(), "failed to encode response", "error", err)
	}
}

// UpdatePost handles PATCH /posts/{id} - updates an existing post.
func (h *PostHandlers) UpdatePost(w http.ResponseWriter, r *http.Request) {
	postID, err := extractPostID(r)
	if err != nil {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Post ID is required")
		return
	}

	userDID := middleware.GetUserDID(r.Context())
	if userDID == "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeAuthFailed)
		WriteError(w, ctx, http.StatusUnauthorized, ErrCodeAuthFailed, "Authentication required")
		return
	}

	var req UpdatePostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Invalid JSON in request body")
		return
	}

	updated, err := h.postService.UpdatePost(postID, userDID, post.UpdatePostInput{
		Text:        req.Text,
		Attachments: req.Attachments,
		Labels:      req.Labels,
	})
	if err != nil {
		code, status := mapError(err)
		ctx := middleware.SetErrorCode(r.Context(), code)
		WriteError(w, ctx, status, code, "Failed to update post")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(updated); err != nil {
		slog.ErrorContext(r.Context(), "failed to encode response", "error", err)
	}
}

// DeletePost handles DELETE /posts/{id} - soft-deletes a post.
func (h *PostHandlers) DeletePost(w http.ResponseWriter, r *http.Request) {
	postID, err := extractPostID(r)
	if err != nil {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Post ID is required")
		return
	}

	userDID := middleware.GetUserDID(r.Context())
	if userDID == "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeAuthFailed)
		WriteError(w, ctx, http.StatusUnauthorized, ErrCodeAuthFailed, "Authentication required")
		return
	}

	if err := h.postService.DeletePost(postID, userDID); err != nil {
		code, status := mapError(err)
		ctx := middleware.SetErrorCode(r.Context(), code)
		WriteError(w, ctx, status, code, "Failed to delete post")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// mapError maps service-level errors to API error codes and HTTP status codes.
func mapError(err error) (string, int) {
	msg := err.Error()
	switch {
	case msg == "either scene_id or event_id must be provided":
		return ErrCodeMissingTarget, http.StatusBadRequest
	case strings.HasPrefix(msg, "invalid post text"):
		return ErrCodeValidation, http.StatusBadRequest
	case strings.HasPrefix(msg, "maximum "):
		return ErrCodeValidation, http.StatusBadRequest
	case strings.HasPrefix(msg, "attachment "):
		return ErrCodeValidation, http.StatusBadRequest
	case msg == "invalid moderation label":
		return ErrCodeValidation, http.StatusBadRequest
	}
	if err == post.ErrPostNotFound {
		return ErrCodeNotFound, http.StatusNotFound
	}
	return ErrCodeInternal, http.StatusInternalServerError
}

// FeedResponse represents the JSON response for feed endpoints.
type FeedResponse struct {
	Posts      []*post.Post     `json:"posts"`
	NextCursor *post.FeedCursor `json:"next_cursor,omitempty"`
}

// parseCursor parses cursor from query parameter.
func parseCursor(cursorStr string) *post.FeedCursor {
	if cursorStr == "" {
		return nil
	}

	parts := strings.Split(cursorStr, ":")
	if len(parts) != 2 {
		return nil
	}

	timestamp, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return nil
	}

	return &post.FeedCursor{
		CreatedAt: time.Unix(0, timestamp),
		ID:        parts[1],
	}
}

// canAccessScene checks if a user can access a scene based on visibility rules.
func (h *PostHandlers) canAccessScene(s *scene.Scene, requesterDID string) (bool, error) {
	if s.IsOwner(requesterDID) {
		return true, nil
	}

	switch s.Visibility {
	case scene.VisibilityPublic:
		return true, nil

	case scene.VisibilityMembersOnly:
		if requesterDID == "" {
			return false, nil
		}
		m, err := h.membershipRepo.GetBySceneAndUser(s.ID, requesterDID)
		if err != nil {
			if err == membership.ErrMembershipNotFound {
				return false, nil
			}
			return false, err
		}
		return m.Status == "active", nil

	case scene.VisibilityHidden:
		return false, nil

	default:
		slog.Warn("unknown visibility mode", "visibility", s.Visibility, "scene_id", s.ID)
		return false, nil
	}
}

// GetSceneFeed handles GET /scenes/{id}/feed - retrieves posts for a scene with pagination.
func (h *PostHandlers) GetSceneFeed(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/scenes/"), "/")
	if len(pathParts) < 2 || pathParts[0] == "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Scene ID is required")
		return
	}
	sceneID := pathParts[0]

	foundScene, err := h.sceneRepo.GetByID(sceneID)
	if err != nil {
		if err == scene.ErrSceneNotFound || err == scene.ErrSceneDeleted {
			slog.DebugContext(r.Context(), "scene not found or deleted", "scene_id", sceneID)
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeNotFound)
			WriteError(w, ctx, http.StatusNotFound, ErrCodeNotFound, "Scene not found")
			return
		}
		slog.ErrorContext(r.Context(), "failed to retrieve scene", "error", err, "scene_id", sceneID)
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to retrieve scene")
		return
	}

	requesterDID := middleware.GetUserDID(r.Context())

	canAccess, err := h.canAccessScene(foundScene, requesterDID)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to check scene access", "error", err, "scene_id", sceneID)
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to check access permissions")
		return
	}

	if !canAccess {
		slog.DebugContext(r.Context(), "scene access denied",
			"scene_id", sceneID,
			"visibility", foundScene.Visibility,
			"requester_did", requesterDID,
			"is_owner", foundScene.IsOwner(requesterDID))
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeNotFound)
		WriteError(w, ctx, http.StatusNotFound, ErrCodeNotFound, "Scene not found")
		return
	}

	limitStr := r.URL.Query().Get("limit")
	cursorStr := r.URL.Query().Get("cursor")

	limit := 20
	if limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err != nil || parsedLimit < 1 {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
			WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, "Invalid limit parameter")
			return
		}
		if parsedLimit > 100 {
			parsedLimit = 100
		}
		limit = parsedLimit
	}

	cursor := parseCursor(cursorStr)

	posts, nextCursor, err := h.postService.ListPostsByScene(sceneID, limit, cursor)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to list scene posts", "error", err, "scene_id", sceneID)
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to retrieve posts")
		return
	}

	response := FeedResponse{
		Posts:      posts,
		NextCursor: nextCursor,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.ErrorContext(r.Context(), "failed to encode response", "error", err)
	}
}

// GetEventFeed handles GET /events/{id}/feed - retrieves posts for an event with pagination.
func (h *PostHandlers) GetEventFeed(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/events/"), "/")
	if len(pathParts) < 2 || pathParts[0] == "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Event ID is required")
		return
	}
	eventID := pathParts[0]

	limitStr := r.URL.Query().Get("limit")
	cursorStr := r.URL.Query().Get("cursor")

	limit := 20
	if limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err != nil || parsedLimit < 1 {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
			WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, "Invalid limit parameter")
			return
		}
		if parsedLimit > 100 {
			parsedLimit = 100
		}
		limit = parsedLimit
	}

	cursor := parseCursor(cursorStr)

	posts, nextCursor, err := h.postService.ListPostsByEvent(eventID, limit, cursor)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to list event posts", "error", err, "event_id", eventID)
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to retrieve posts")
		return
	}

	response := FeedResponse{
		Posts:      posts,
		NextCursor: nextCursor,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.ErrorContext(r.Context(), "failed to encode response", "error", err)
	}
}

// RegisterPostRoutes registers all post-related routes on the given mux.
func RegisterPostRoutes(mux *http.ServeMux, deps *RouteDeps, h *PostHandlers) {
	mux.HandleFunc("/posts", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			h.CreatePost(w, r)
		default:
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
			WriteError(w, ctx, http.StatusMethodNotAllowed, ErrCodeBadRequest, "Method not allowed")
		}
	})

	mux.HandleFunc("/posts/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPatch:
			h.UpdatePost(w, r)
		case http.MethodDelete:
			h.DeletePost(w, r)
		default:
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
			WriteError(w, ctx, http.StatusMethodNotAllowed, ErrCodeBadRequest, "Method not allowed")
		}
	})
}