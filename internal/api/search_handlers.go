// Package api provides HTTP handlers for the Subcults API.
package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/onnwee/subcults/internal/middleware"
	"github.com/onnwee/subcults/internal/scene"
	"github.com/onnwee/subcults/internal/trust"
)

// SearchHandlers holds dependencies for search HTTP handlers.
type SearchHandlers struct {
	sceneRepo scene.SceneRepository
	trustStore TrustScoreStore
}

// NewSearchHandlers creates a new SearchHandlers instance.
func NewSearchHandlers(sceneRepo scene.SceneRepository, trustStore TrustScoreStore) *SearchHandlers {
	return &SearchHandlers{
		sceneRepo:  sceneRepo,
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
	ID              string        `json:"id"`
	Name            string        `json:"name"`
	Description     string        `json:"description,omitempty"`
	JitteredPoint   *scene.Point  `json:"jittered_centroid,omitempty"` // Always jittered for privacy
	CoarseGeohash   string        `json:"coarse_geohash"`
	Tags            []string      `json:"tags,omitempty"`
	Visibility      string        `json:"visibility"`
	TrustScore      *float64      `json:"trust_score,omitempty"` // Only if trust ranking enabled
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
	
	// Get bbox parameters
	bboxStr := query.Get("bbox")
	if bboxStr == "" && q == "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, "At least one of 'q' or 'bbox' must be provided")
		return
	}
	
	// Parse bbox if provided
	var minLng, minLat, maxLng, maxLat float64
	if bboxStr != "" {
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
	} else {
		// No bbox provided, use world bounds
		minLng = -180
		minLat = -90
		maxLng = 180
		maxLat = 90
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
	
	// Get trust scores if trust ranking is enabled
	var trustScores map[string]float64
	if trust.IsRankingEnabled() && h.trustStore != nil {
		// For now, we'll pass nil and let the ranking use default scores
		// In production, this would fetch trust scores for scenes in the result set
		trustScores = nil
	}
	
	// Execute search
	results, nextCursor, err := h.sceneRepo.SearchScenes(scene.SceneSearchOptions{
		MinLng:      minLng,
		MinLat:      minLat,
		MaxLng:      maxLng,
		MaxLat:      maxLat,
		Query:       q,
		Limit:       limit,
		Cursor:      cursor,
		TrustScores: trustScores,
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
		
		// Include trust score if available and enabled
		if trust.IsRankingEnabled() && trustScores != nil {
			if ts, ok := trustScores[s.ID]; ok {
				result.TrustScore = &ts
			}
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

// applyJitter applies deterministic jitter to a point for privacy.
// This is a simple implementation for the in-memory repository.
// In production, this would use the geo.ApplyJitter function with proper geohash-based jitter.
func applyJitter(point *scene.Point) *scene.Point {
	// Simple jitter: add small random offset based on coordinates (deterministic)
	// This prevents exact location exposure while keeping results stable
	// For production, use proper geohash-based jitter from geo package
	
	// Use coordinates as seed for deterministic jitter
	seed := int64(point.Lat*1000000 + point.Lng*1000000)
	offset := float64((seed % 1000)) / 10000.0 // ~0.01 degree offset
	
	return &scene.Point{
		Lat: point.Lat + offset,
		Lng: point.Lng + offset,
	}
}
