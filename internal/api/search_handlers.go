// Package api provides HTTP handlers for the Subcults API.
package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/onnwee/subcults/internal/middleware"
	"github.com/onnwee/subcults/internal/post"
	"github.com/onnwee/subcults/internal/scene"
)

// SearchHandlers holds dependencies for search HTTP handlers.
type SearchHandlers struct {
	sceneRepo  scene.SceneRepository
	postRepo   post.PostRepository
	trustStore TrustScoreStore
}

// NewSearchHandlers creates a new SearchHandlers instance.
func NewSearchHandlers(sceneRepo scene.SceneRepository, postRepo post.PostRepository, trustStore TrustScoreStore) *SearchHandlers {
	return &SearchHandlers{
		sceneRepo:  sceneRepo,
		postRepo:   postRepo,
		trustStore: trustStore,
	}
}

// SceneSearchResponse represents the response for scene search.
type SceneSearchResponse struct {
	Results    []*SceneSearchResult `json:"results"`
	NextCursor string               `json:"next_cursor,omitempty"`
	Count      int                  `json:"count"`
}

// SceneSearchResult represents a minimal scene result for search.
type SceneSearchResult struct {
	ID            string       `json:"id"`
	Name          string       `json:"name"`
	Description   string       `json:"description,omitempty"`
	JitteredPoint *scene.Point `json:"jittered_centroid,omitempty"` // Always jittered for privacy
	CoarseGeohash string       `json:"coarse_geohash"`
	Tags          []string     `json:"tags,omitempty"`
	Visibility    string       `json:"visibility"`
	TrustScore    *float64     `json:"trust_score,omitempty"` // Only if trust ranking enabled
}

// Constants for bbox validation
const (
	MaxBboxAreaDegrees = 10.0 // Max bbox area in square degrees (~1000km x 1000km at equator)
	MaxSearchLimit     = 50   // Max results per page
	DefaultSearchLimit = 20   // Default results if not specified
)

// SearchScenes handles GET /search/scenes - searches for scenes with ranking and pagination.
func (h *SearchHandlers) SearchScenes(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	query := r.URL.Query()

	// Get text search query (optional)
	q := strings.TrimSpace(query.Get("q"))

	// Get bbox parameters (required)
	bboxStr := query.Get("bbox")
	if bboxStr == "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, "bbox parameter is required")
		return
	}

	// Parse bbox
	var minLng, minLat, maxLng, maxLat float64
	{
		parts := strings.Split(bboxStr, ",")
		if len(parts) != 4 {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
			WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, "bbox must be in format: minLng,minLat,maxLng,maxLat")
			return
		}

		var err error
		minLng, err = strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		if err != nil {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
			WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, "Invalid minLng in bbox")
			return
		}

		minLat, err = strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err != nil {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
			WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, "Invalid minLat in bbox")
			return
		}

		maxLng, err = strconv.ParseFloat(strings.TrimSpace(parts[2]), 64)
		if err != nil {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
			WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, "Invalid maxLng in bbox")
			return
		}

		maxLat, err = strconv.ParseFloat(strings.TrimSpace(parts[3]), 64)
		if err != nil {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
			WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, "Invalid maxLat in bbox")
			return
		}

		// Validate bbox coordinates
		if minLng < -180 || minLng > 180 || maxLng < -180 || maxLng > 180 {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
			WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, "Longitude must be between -180 and 180")
			return
		}

		if minLat < -90 || minLat > 90 || maxLat < -90 || maxLat > 90 {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
			WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, "Latitude must be between -90 and 90")
			return
		}

		if minLng >= maxLng {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
			WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, "minLng must be less than maxLng")
			return
		}

		if minLat >= maxLat {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
			WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, "minLat must be less than maxLat")
			return
		}

		// Validate bbox area (prevent wide scans)
		area := (maxLng - minLng) * (maxLat - minLat)
		if area > MaxBboxAreaDegrees {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
			WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, fmt.Sprintf("bbox area too large (max %.1f square degrees)", MaxBboxAreaDegrees))
			return
		}
	}

	// Get pagination parameters
	cursor := query.Get("cursor")

	limit := DefaultSearchLimit
	if limitStr := query.Get("limit"); limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit < 1 {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
			WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, "limit must be a positive integer")
			return
		}
		if limit > MaxSearchLimit {
			limit = MaxSearchLimit
		}
	}

	// Execute search
	// Note: Trust scoring is not yet implemented. When implemented, trust scores
	// should be fetched for the requester's trust graph and passed to SearchScenes.
	results, nextCursor, err := h.sceneRepo.SearchScenes(scene.SceneSearchOptions{
		MinLng:      minLng,
		MinLat:      minLat,
		MaxLng:      maxLng,
		MaxLat:      maxLat,
		Query:       q,
		Limit:       limit,
		Cursor:      cursor,
		TrustScores: nil, // Trust scoring not yet implemented
	})

	if err != nil {
		slog.ErrorContext(r.Context(), "failed to search scenes", "error", err, "query", q, "bbox", bboxStr)
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to search scenes")
		return
	}

	// Convert to search results with jittered coordinates
	searchResults := make([]*SceneSearchResult, 0, len(results))
	for _, s := range results {
		result := &SceneSearchResult{
			ID:            s.ID,
			Name:          s.Name,
			Description:   s.Description,
			CoarseGeohash: s.CoarseGeohash,
			Tags:          s.Tags,
			Visibility:    s.Visibility,
		}

		// Apply jitter to coordinates for privacy
		// Even if allow_precise is true, we jitter for public search results
		if s.PrecisePoint != nil {
			result.JitteredPoint = applyJitter(s.PrecisePoint)
		}

		searchResults = append(searchResults, result)
	}

	// Build response
	response := SceneSearchResponse{
		Results:    searchResults,
		NextCursor: nextCursor,
		Count:      len(searchResults),
	}

	// Return results
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.ErrorContext(r.Context(), "failed to encode search response", "error", err)
		return
	}
}

// applyJitter applies deterministic geohash-based jitter to a point for privacy.
// Uses the point's geohash to generate a stable offset that prevents exact location
// exposure while maintaining determinism (same coordinates = same jittered result).
func applyJitter(point *scene.Point) *scene.Point {
	// Calculate geohash at precision 8 (approximately 20m x 20m cell)
	// This provides stable jitter based on the location's geohash cell
	geohash := encodeGeohash(point.Lat, point.Lng, 8)

	// Use geohash to generate deterministic offset
	// Hash the geohash string to get a numeric seed
	var hash int64
	for i, c := range geohash {
		hash = hash*31 + int64(c) + int64(i)
	}

	// Ensure positive seed
	if hash < 0 {
		hash = -hash
	}

	// Generate offset in range [0.005, 0.015] degrees (~500m to 1.5km at equator)
	// This is sufficient to hide exact location while keeping scenes discoverable
	latOffset := 0.005 + float64(hash%1000)/100000.0
	lngOffset := 0.005 + float64((hash/1000)%1000)/100000.0

	// Apply offset in a deterministic direction based on hash
	latSign := 1.0
	if (hash/1000000)%2 == 0 {
		latSign = -1.0
	}
	lngSign := 1.0
	if (hash/2000000)%2 == 0 {
		lngSign = -1.0
	}

	return &scene.Point{
		Lat: point.Lat + latOffset*latSign,
		Lng: point.Lng + lngOffset*lngSign,
	}
}

// encodeGeohash encodes a lat/lng coordinate to a geohash string.
// This is a simplified implementation for jitter calculation.
// For production, use a proper geohash library.
func encodeGeohash(lat, lng float64, precision int) string {
	const base32 = "0123456789bcdefghjkmnpqrstuvwxyz"

	minLat, maxLat := -90.0, 90.0
	minLng, maxLng := -180.0, 180.0

	var geohash strings.Builder
	bits := 0
	bit := 0
	ch := 0

	for geohash.Len() < precision {
		if bits%2 == 0 {
			// longitude
			mid := (minLng + maxLng) / 2
			if lng > mid {
				ch |= (1 << (4 - bit))
				minLng = mid
			} else {
				maxLng = mid
			}
		} else {
			// latitude
			mid := (minLat + maxLat) / 2
			if lat > mid {
				ch |= (1 << (4 - bit))
				minLat = mid
			} else {
				maxLat = mid
			}
		}

		bits++
		bit++

		if bit == 5 {
			geohash.WriteByte(base32[ch])
			bit = 0
			ch = 0
		}
	}

	return geohash.String()
}

// PostSearchResponse represents the response for post search.
type PostSearchResponse struct {
	Results    []*PostSearchResult `json:"results"`
	NextCursor string              `json:"next_cursor,omitempty"`
	Count      int                 `json:"count"`
}

// PostSearchResult represents a minimal post result for search.
type PostSearchResult struct {
	ID         string   `json:"id"`
	Excerpt    string   `json:"excerpt"` // First 160 chars
	SceneID    *string  `json:"scene_id,omitempty"`
	TrustScore *float64 `json:"trust_score,omitempty"` // Only if trust ranking enabled
	CreatedAt  string   `json:"created_at"`            // ISO 8601 format
}

// SearchPosts handles GET /search/posts - searches for posts with text relevance and scene filter.
func (h *SearchHandlers) SearchPosts(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	query := r.URL.Query()

	// Get text search query (required)
	q := strings.TrimSpace(query.Get("q"))
	if q == "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, "q parameter is required")
		return
	}

	// Get optional scene filter
	var sceneID *string
	if sceneIDStr := query.Get("scene_id"); sceneIDStr != "" {
		sceneID = &sceneIDStr
	}

	// Get pagination parameters
	cursor := query.Get("cursor")

	limit := DefaultSearchLimit
	if limitStr := query.Get("limit"); limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit < 1 {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
			WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, "limit must be a positive integer")
			return
		}
		if limit > MaxSearchLimit {
			limit = MaxSearchLimit
		}
	}

	// Trust scores are not yet implemented for post search
	// Pass nil to use text relevance only
	var trustScores map[string]float64 = nil

	// Execute search
	results, nextCursor, err := h.postRepo.SearchPosts(q, sceneID, limit, cursor, trustScores)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to search posts", "error", err, "query", q, "scene_id", sceneID)
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to search posts")
		return
	}

	// Convert to search results with excerpts
	searchResults := make([]*PostSearchResult, 0, len(results))
	for _, p := range results {
		result := &PostSearchResult{
			ID:        p.ID,
			Excerpt:   makeExcerpt(p.Text, 160),
			SceneID:   p.SceneID,
			CreatedAt: p.CreatedAt.Format("2006-01-02T15:04:05Z07:00"), // ISO 8601
		}

		// Note: Trust score integration is not yet implemented for post search
		// trust_score field will always be nil and is omitted from JSON response

		searchResults = append(searchResults, result)
	}

	// Build response
	response := PostSearchResponse{
		Results:    searchResults,
		NextCursor: nextCursor,
		Count:      len(searchResults),
	}

	// Return results
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.ErrorContext(r.Context(), "failed to encode search response", "error", err)
		return
	}
}

// makeExcerpt creates a text excerpt of the specified length.
// Truncates at word boundary if possible.
func makeExcerpt(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}

	// Try to truncate at word boundary
	truncated := text[:maxLen]
	lastSpace := strings.LastIndex(truncated, " ")
	if lastSpace > maxLen/2 {
		// If we found a space in the second half, use it
		return truncated[:lastSpace] + "..."
	}

	// Otherwise just truncate at max length
	return truncated + "..."
}
