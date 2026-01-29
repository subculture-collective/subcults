package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/livekit/protocol/livekit"
	livekitservice "github.com/onnwee/subcults/internal/livekit"
	"github.com/onnwee/subcults/internal/middleware"
	"github.com/onnwee/subcults/internal/stream"
)

// QualityMetricsHandler handles quality metrics API endpoints.
type QualityMetricsHandler struct {
	roomService       *livekitservice.RoomService
	metricsRepo       stream.QualityMetricsRepository
	streamRepo        stream.SessionRepository
	streamMetrics     *stream.Metrics
}

// NewQualityMetricsHandler creates a new QualityMetricsHandler.
func NewQualityMetricsHandler(
	roomService *livekitservice.RoomService,
	metricsRepo stream.QualityMetricsRepository,
	streamRepo stream.SessionRepository,
	streamMetrics *stream.Metrics,
) *QualityMetricsHandler {
	return &QualityMetricsHandler{
		roomService:   roomService,
		metricsRepo:   metricsRepo,
		streamRepo:    streamRepo,
		streamMetrics: streamMetrics,
	}
}

// GetStreamQualityMetrics retrieves quality metrics for a stream session.
// GET /streams/{id}/quality-metrics
func (h *QualityMetricsHandler) GetStreamQualityMetrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Extract stream ID from URL path
	// Expected: /streams/{id}/quality-metrics
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/streams/"), "/")
	if len(pathParts) < 2 || pathParts[0] == "" {
		ctx = middleware.SetErrorCode(ctx, ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Invalid URL path")
		return
	}
	streamID := pathParts[0]

	// Parse optional limit parameter (default: 100)
	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err == nil && parsedLimit > 0 && parsedLimit <= 1000 {
			limit = parsedLimit
		}
	}

	// Get quality metrics
	metrics, err := h.metricsRepo.GetMetricsBySession(streamID, limit)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get quality metrics", "stream_id", streamID, "error", err)
		ctx = middleware.SetErrorCode(ctx, ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to retrieve quality metrics")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"stream_id": streamID,
		"metrics":   metrics,
		"count":     len(metrics),
	})
}

// GetParticipantQualityMetrics retrieves quality metrics for a specific participant.
// GET /streams/{id}/participants/{participant_id}/quality-metrics
func (h *QualityMetricsHandler) GetParticipantQualityMetrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Extract stream ID and participant ID from URL path
	// Expected: /streams/{id}/participants/{participant_id}/quality-metrics
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/streams/"), "/")
	if len(pathParts) < 4 || pathParts[0] == "" || pathParts[2] == "" {
		ctx = middleware.SetErrorCode(ctx, ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Invalid URL path")
		return
	}
	streamID := pathParts[0]
	participantID := pathParts[2]

	// Get latest metrics
	metrics, err := h.metricsRepo.GetLatestMetrics(streamID, participantID)
	if err != nil {
		if errors.Is(err, stream.ErrQualityMetricsNotFound) {
			ctx = middleware.SetErrorCode(ctx, ErrCodeNotFound)
			WriteError(w, ctx, http.StatusNotFound, ErrCodeNotFound, "Quality metrics not found")
			return
		}
		slog.ErrorContext(ctx, "failed to get participant quality metrics", 
			"stream_id", streamID, 
			"participant_id", participantID, 
			"error", err)
		ctx = middleware.SetErrorCode(ctx, ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to retrieve quality metrics")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

// CollectStreamQualityMetrics collects quality metrics from LiveKit for all participants in a stream.
// POST /streams/{id}/quality-metrics/collect
// This endpoint is typically called periodically by a background job or external monitor.
func (h *QualityMetricsHandler) CollectStreamQualityMetrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Extract stream ID from URL path
	// Expected: /streams/{id}/quality-metrics/collect
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/streams/"), "/")
	if len(pathParts) < 2 || pathParts[0] == "" {
		ctx = middleware.SetErrorCode(ctx, ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Invalid URL path")
		return
	}
	streamID := pathParts[0]

	// Get stream session
	session, err := h.streamRepo.GetByID(streamID)
	if err != nil {
		if errors.Is(err, stream.ErrStreamNotFound) {
			ctx = middleware.SetErrorCode(ctx, ErrCodeNotFound)
			WriteError(w, ctx, http.StatusNotFound, ErrCodeNotFound, "Stream not found")
			return
		}
		slog.ErrorContext(ctx, "failed to get stream", "stream_id", streamID, "error", err)
		ctx = middleware.SetErrorCode(ctx, ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to retrieve stream")
		return
	}

	// Check if stream is active
	if session.EndedAt != nil {
		ctx = middleware.SetErrorCode(ctx, ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Stream has ended")
		return
	}

	// Get all participants from LiveKit
	participants, err := h.roomService.GetAllParticipantStats(ctx, session.RoomName)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get participant stats from LiveKit", 
			"room_name", session.RoomName, 
			"error", err)
		ctx = middleware.SetErrorCode(ctx, ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to retrieve participant stats")
		return
	}

	// Process and store metrics for each participant
	var recordedCount int
	var alertsTriggered int
	measuredAt := time.Now()

	for _, participant := range participants {
		metrics := extractQualityMetrics(streamID, participant, measuredAt)
		
		// Store metrics
		if err := h.metricsRepo.RecordMetrics(metrics); err != nil {
			slog.ErrorContext(ctx, "failed to record quality metrics", 
				"participant_id", participant.Identity, 
				"error", err)
			continue
		}
		recordedCount++

		// Update Prometheus metrics
		h.updatePrometheusMetrics(metrics)

		// Check for quality alerts
		if metrics.HasPoorNetworkQuality() {
			alertsTriggered++
			h.streamMetrics.IncQualityAlerts()
			
			slog.WarnContext(ctx, "poor network quality detected",
				"stream_id", streamID,
				"participant_id", participant.Identity,
				"packet_loss", metrics.PacketLossPercent,
				"jitter", metrics.JitterMs,
				"rtt", metrics.RTTMs)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"stream_id":        streamID,
		"participants":     len(participants),
		"metrics_recorded": recordedCount,
		"alerts_triggered": alertsTriggered,
		"measured_at":      measuredAt,
	})
}

// GetHighPacketLossParticipants returns participants with recent high packet loss.
// GET /streams/{id}/quality-metrics/high-packet-loss
func (h *QualityMetricsHandler) GetHighPacketLossParticipants(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Extract stream ID from URL path
	// Expected: /streams/{id}/quality-metrics/high-packet-loss
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/streams/"), "/")
	if len(pathParts) < 2 || pathParts[0] == "" {
		ctx = middleware.SetErrorCode(ctx, ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Invalid URL path")
		return
	}
	streamID := pathParts[0]

	// Parse optional time window parameter (default: 5 minutes)
	sinceMinutes := 5
	if sinceStr := r.URL.Query().Get("since_minutes"); sinceStr != "" {
		parsedSince, err := strconv.Atoi(sinceStr)
		if err == nil && parsedSince > 0 && parsedSince <= 60 {
			sinceMinutes = parsedSince
		}
	}

	participants, err := h.metricsRepo.GetParticipantsWithHighPacketLoss(streamID, sinceMinutes)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get participants with high packet loss", 
			"stream_id", streamID, 
			"error", err)
		ctx = middleware.SetErrorCode(ctx, ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to retrieve participants")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"stream_id":      streamID,
		"since_minutes":  sinceMinutes,
		"participants":   participants,
		"count":          len(participants),
	})
}

// extractQualityMetrics extracts quality metrics from LiveKit ParticipantInfo.
func extractQualityMetrics(streamID string, participant *livekit.ParticipantInfo, measuredAt time.Time) *stream.QualityMetrics {
	metrics := &stream.QualityMetrics{
		StreamSessionID: streamID,
		ParticipantID:   participant.Identity,
		MeasuredAt:      measuredAt,
	}

	// Extract stats from audio tracks
	for _, track := range participant.Tracks {
		if track.Type == livekit.TrackType_AUDIO {
			// Extract quality metrics from track
			// Note: LiveKit's ParticipantInfo includes track statistics
			// The exact fields depend on the LiveKit protocol version
			
			// Bitrate (convert from bps to kbps)
			if track.Muted == false {
				// Placeholder: In real implementation, extract from track stats
				// For now, we'll leave these nil to be populated by actual LiveKit data
			}
		}
	}

	return metrics
}

// updatePrometheusMetrics updates Prometheus metrics with quality data.
func (h *QualityMetricsHandler) updatePrometheusMetrics(metrics *stream.QualityMetrics) {
	if metrics.BitrateKbps != nil {
		h.streamMetrics.ObserveAudioBitrate(*metrics.BitrateKbps)
	}
	if metrics.JitterMs != nil {
		h.streamMetrics.ObserveAudioJitter(*metrics.JitterMs)
	}
	if metrics.PacketLossPercent != nil {
		h.streamMetrics.ObserveAudioPacketLoss(*metrics.PacketLossPercent)
	}
	if metrics.AudioLevel != nil {
		h.streamMetrics.ObserveAudioLevel(*metrics.AudioLevel)
	}
	if metrics.RTTMs != nil {
		h.streamMetrics.ObserveNetworkRTT(*metrics.RTTMs)
	}
}
