// Package stream provides SQL-backed implementations of the stream domain repositories.
package stream

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// ──────────────────────────────────────────────
// SQLSessionRepository
// ──────────────────────────────────────────────

type SQLSessionRepository struct{ db *sql.DB }

func NewSQLSessionRepository(db *sql.DB) *SQLSessionRepository {
	return &SQLSessionRepository{db: db}
}

type rowScanner interface{ Scan(...any) error }

const sessionColumns = `id::text,COALESCE(scene_id::text,''),COALESCE(event_id::text,''),room_name,host_did,
participant_count,active_participant_count,COALESCE(record_did,''),COALESCE(record_rkey,''),
join_count,leave_count,is_locked,COALESCE(featured_participant,''),started_at,ended_at`

func scanSession(row rowScanner) (*Session, error) {
	var s Session
	var sceneID, eventID, rdID, rrKey, featPart sql.NullString
	var endedAt sql.NullTime
	err := row.Scan(&s.ID, &sceneID, &eventID, &s.RoomName, &s.HostDID,
		&s.ParticipantCount, &s.ActiveParticipantCount, &rdID, &rrKey,
		&s.JoinCount, &s.LeaveCount, &s.IsLocked, &featPart,
		&s.StartedAt, &endedAt)
	if err != nil {
		return nil, err
	}
	s.SceneID = stringPointer(sceneID)
	s.EventID = stringPointer(eventID)
	s.RecordDID = stringPointer(rdID)
	s.RecordRKey = stringPointer(rrKey)
	s.FeaturedParticipant = stringPointer(featPart)
	s.EndedAt = timePointer(endedAt)
	return &s, nil
}

func stringPointer(value sql.NullString) *string {
	if !value.Valid || value.String == "" {
		return nil
	}
	result := value.String
	return &result
}

func timePointer(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time
	return &result
}

func nullString(value *string) any {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil
	}
	return *value
}

func nullTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return *value
}

func (r *SQLSessionRepository) Upsert(session *Session) (*UpsertResult, error) {
	if session.RecordDID != nil && *session.RecordDID != "" && session.RecordRKey != nil && *session.RecordRKey != "" {
		existing, err := r.GetByRecordKey(*session.RecordDID, *session.RecordRKey)
		if err == nil {
			session.ID = existing.ID
			if session.StartedAt.IsZero() {
				session.StartedAt = existing.StartedAt
			}
			if _, err = r.db.Exec(`UPDATE stream_sessions SET scene_id=$2::uuid,event_id=$3::uuid,room_name=$4,host_did=$5,ended_at=$6,is_locked=$7,featured_participant=$8 WHERE id=$1::uuid`,
				session.ID, nullString(session.SceneID), nullString(session.EventID), session.RoomName, session.HostDID, nullTime(session.EndedAt), session.IsLocked, nullString(session.FeaturedParticipant)); err != nil {
				return nil, fmt.Errorf("stream upsert: %w", err)
			}
			return &UpsertResult{ID: session.ID}, nil
		}
		if !errors.Is(err, ErrStreamNotFound) {
			return nil, err
		}
	}
	if session.ID == "" {
		session.ID = uuid.New().String()
	}
	if session.StartedAt.IsZero() {
		session.StartedAt = time.Now()
	}
	_, err := r.db.Exec(`INSERT INTO stream_sessions(id,scene_id,event_id,room_name,host_did,is_locked,featured_participant,started_at,ended_at,record_did,record_rkey)
		VALUES($1::uuid,$2::uuid,$3::uuid,$4,$5,$6,$7,$8,$9,$10,$11)`,
		session.ID, nullString(session.SceneID), nullString(session.EventID), session.RoomName, session.HostDID,
		session.IsLocked, nullString(session.FeaturedParticipant), session.StartedAt, nullTime(session.EndedAt),
		nullString(session.RecordDID), nullString(session.RecordRKey))
	if err != nil {
		return nil, fmt.Errorf("stream insert: %w", err)
	}
	return &UpsertResult{Inserted: true, ID: session.ID}, nil
}

func (r *SQLSessionRepository) GetByID(id string) (*Session, error) {
	s, err := scanSession(r.db.QueryRow(`SELECT `+sessionColumns+` FROM stream_sessions WHERE id=$1::uuid`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrStreamNotFound
	}
	return s, err
}

func (r *SQLSessionRepository) GetByRecordKey(did, rkey string) (*Session, error) {
	s, err := scanSession(r.db.QueryRow(`SELECT `+sessionColumns+` FROM stream_sessions WHERE record_did=$1 AND record_rkey=$2`, did, rkey))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrStreamNotFound
	}
	return s, err
}

func (r *SQLSessionRepository) CreateStreamSession(sceneID *string, eventID *string, hostDID string) (string, string, error) {
	if hostDID == "" {
		return "", "", errors.New("hostDID must not be empty")
	}
	if (sceneID == nil || *sceneID == "") && (eventID == nil || *eventID == "") {
		return "", "", errors.New("either scene_id or event_id must be provided")
	}
	now := time.Now()
	timestamp := now.Unix()
	var roomName string
	if sceneID != nil && *sceneID != "" {
		roomName = fmt.Sprintf("scene-%s-%d", *sceneID, timestamp)
	} else {
		roomName = fmt.Sprintf("event-%s-%d", *eventID, timestamp)
	}
	id := uuid.New().String()
	_, err := r.db.Exec(`INSERT INTO stream_sessions(id,scene_id,event_id,room_name,host_did,started_at)
		VALUES($1::uuid,$2::uuid,$3::uuid,$4,$5,$6)`,
		id, nullString(sceneID), nullString(eventID), roomName, hostDID, now)
	if err != nil {
		return "", "", fmt.Errorf("create stream session: %w", err)
	}
	return id, roomName, nil
}

func (r *SQLSessionRepository) EndStreamSession(id string) error {
	result, err := r.db.Exec(`UPDATE stream_sessions SET ended_at=NOW() WHERE id=$1::uuid AND ended_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("end stream session: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		var exists bool
		r.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM stream_sessions WHERE id=$1::uuid)`, id).Scan(&exists)
		if exists {
			return nil // already ended
		}
		return ErrStreamNotFound
	}
	return nil
}

func (r *SQLSessionRepository) RecordJoin(id string) error {
	result, err := r.db.Exec(`UPDATE stream_sessions SET join_count=join_count+1 WHERE id=$1::uuid`, id)
	if err != nil {
		return fmt.Errorf("record join: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrStreamNotFound
	}
	return nil
}

func (r *SQLSessionRepository) RecordLeave(id string) error {
	result, err := r.db.Exec(`UPDATE stream_sessions SET leave_count=leave_count+1 WHERE id=$1::uuid`, id)
	if err != nil {
		return fmt.Errorf("record leave: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrStreamNotFound
	}
	return nil
}

func (r *SQLSessionRepository) UpdateActiveParticipantCount(id string, count int) error {
	result, err := r.db.Exec(`UPDATE stream_sessions SET active_participant_count=$2 WHERE id=$1::uuid`, id, count)
	if err != nil {
		return fmt.Errorf("update active participant count: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrStreamNotFound
	}
	return nil
}

func (r *SQLSessionRepository) SetLockStatus(id string, locked bool) error {
	result, err := r.db.Exec(`UPDATE stream_sessions SET is_locked=$2 WHERE id=$1::uuid`, id, locked)
	if err != nil {
		return fmt.Errorf("set lock status: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrStreamNotFound
	}
	return nil
}

func (r *SQLSessionRepository) SetFeaturedParticipant(id string, participantID *string) error {
	result, err := r.db.Exec(`UPDATE stream_sessions SET featured_participant=$2 WHERE id=$1::uuid`, id, nullString(participantID))
	if err != nil {
		return fmt.Errorf("set featured participant: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrStreamNotFound
	}
	return nil
}

func (r *SQLSessionRepository) HasActiveStreamForScene(sceneID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM stream_sessions WHERE scene_id=$1::uuid AND ended_at IS NULL)`, sceneID).Scan(&exists)
	return exists, err
}

func (r *SQLSessionRepository) HasActiveStreamsForScenes(sceneIDs []string) (map[string]bool, error) {
	result := make(map[string]bool, len(sceneIDs))
	for _, id := range sceneIDs {
		result[id] = false
	}
	if len(sceneIDs) == 0 {
		return result, nil
	}
	rows, err := r.db.Query(`SELECT scene_id::text FROM stream_sessions WHERE scene_id=ANY($1::uuid[]) AND ended_at IS NULL`, pq.Array(sceneIDs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var sid string
		if err := rows.Scan(&sid); err != nil {
			return nil, err
		}
		result[sid] = true
	}
	return result, rows.Err()
}

func (r *SQLSessionRepository) GetActiveStreamForEvent(eventID string) (*ActiveStreamInfo, error) {
	var info ActiveStreamInfo
	err := r.db.QueryRow(`SELECT id::text,room_name,started_at FROM stream_sessions WHERE event_id=$1::uuid AND ended_at IS NULL ORDER BY started_at DESC LIMIT 1`, eventID).Scan(&info.StreamSessionID, &info.RoomName, &info.StartedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &info, nil
}

func (r *SQLSessionRepository) GetActiveStreamsForEvents(eventIDs []string) (map[string]*ActiveStreamInfo, error) {
	result := make(map[string]*ActiveStreamInfo, len(eventIDs))
	if len(eventIDs) == 0 {
		return result, nil
	}
	rows, err := r.db.Query(`SELECT DISTINCT ON(event_id) event_id::text,id::text,room_name,started_at FROM stream_sessions WHERE event_id=ANY($1::uuid[]) AND ended_at IS NULL ORDER BY event_id,started_at DESC`, pq.Array(eventIDs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var eid string
		var info ActiveStreamInfo
		if err := rows.Scan(&eid, &info.StreamSessionID, &info.RoomName, &info.StartedAt); err != nil {
			return nil, err
		}
		result[eid] = &info
	}
	return result, rows.Err()
}

// ──────────────────────────────────────────────
// SQLParticipantRepository
// ──────────────────────────────────────────────

type SQLParticipantRepository struct{ db *sql.DB }

func NewSQLParticipantRepository(db *sql.DB) *SQLParticipantRepository {
	return &SQLParticipantRepository{db: db}
}

const participantColumns = `id::text,stream_session_id::text,participant_id,user_did,joined_at,left_at,reconnection_count,created_at,updated_at`

func scanParticipant(row rowScanner) (*Participant, error) {
	var p Participant
	var leftAt sql.NullTime
	err := row.Scan(&p.ID, &p.StreamSessionID, &p.ParticipantID, &p.UserDID, &p.JoinedAt, &leftAt, &p.ReconnectionCount, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	p.LeftAt = timePointer(leftAt)
	return &p, nil
}

func (r *SQLParticipantRepository) RecordJoin(streamSessionID, participantID, userDID string) (*Participant, bool, error) {
	now := time.Now()

	// Check for reconnection count
	var reconnectionCount int
	err := r.db.QueryRow(`SELECT COALESCE(MAX(reconnection_count),0) FROM stream_participants WHERE stream_session_id=$1::uuid AND participant_id=$2`, streamSessionID, participantID).Scan(&reconnectionCount)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, false, fmt.Errorf("participant join check: %w", err)
	}
	if reconnectionCount > 0 {
		reconnectionCount++
	}

	id := uuid.New().String()
	_, err = r.db.Exec(`INSERT INTO stream_participants(id,stream_session_id,participant_id,user_did,joined_at,reconnection_count)
		VALUES($1::uuid,$2::uuid,$3,$4,$5,$6)`,
		id, streamSessionID, participantID, userDID, now, reconnectionCount)
	if err != nil {
		// If duplicate active participant, check for ErrParticipantAlreadyActive
		if isUniqueViolation(err) {
			return nil, false, ErrParticipantAlreadyActive
		}
		return nil, false, fmt.Errorf("record participant join: %w", err)
	}

	p := &Participant{
		ID:                id,
		StreamSessionID:   streamSessionID,
		ParticipantID:     participantID,
		UserDID:           userDID,
		JoinedAt:          now,
		LeftAt:            nil,
		ReconnectionCount: reconnectionCount,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	// Update active count
	r.updateActiveCount(streamSessionID)

	return p, reconnectionCount > 0, nil
}

func (r *SQLParticipantRepository) RecordLeave(streamSessionID, participantID string) error {
	result, err := r.db.Exec(`UPDATE stream_participants SET left_at=NOW(),updated_at=NOW() WHERE stream_session_id=$1::uuid AND participant_id=$2 AND left_at IS NULL`, streamSessionID, participantID)
	if err != nil {
		return fmt.Errorf("record participant leave: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrParticipantNotFound
	}
	r.updateActiveCount(streamSessionID)
	return nil
}

func (r *SQLParticipantRepository) GetActiveParticipants(streamSessionID string) ([]*Participant, error) {
	rows, err := r.db.Query(`SELECT `+participantColumns+` FROM stream_participants WHERE stream_session_id=$1::uuid AND left_at IS NULL ORDER BY joined_at`, streamSessionID)
	if err != nil {
		return nil, fmt.Errorf("get active participants: %w", err)
	}
	defer rows.Close()
	var result []*Participant
	for rows.Next() {
		p, err := scanParticipant(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

func (r *SQLParticipantRepository) GetParticipantHistory(streamSessionID string) ([]*Participant, error) {
	rows, err := r.db.Query(`SELECT `+participantColumns+` FROM stream_participants WHERE stream_session_id=$1::uuid ORDER BY joined_at DESC`, streamSessionID)
	if err != nil {
		return nil, fmt.Errorf("get participant history: %w", err)
	}
	defer rows.Close()
	var result []*Participant
	for rows.Next() {
		p, err := scanParticipant(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

func (r *SQLParticipantRepository) GetActiveCount(streamSessionID string) (int, error) {
	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM stream_participants WHERE stream_session_id=$1::uuid AND left_at IS NULL`, streamSessionID).Scan(&count)
	return count, err
}

func (r *SQLParticipantRepository) UpdateSessionParticipantCount(streamSessionID string, count int) error {
	_, err := r.db.Exec(`UPDATE stream_sessions SET active_participant_count=$2 WHERE id=$1::uuid`, streamSessionID, count)
	return err
}

func (r *SQLParticipantRepository) updateActiveCount(streamSessionID string) {
	count, err := r.GetActiveCount(streamSessionID)
	if err != nil {
		return
	}
	r.UpdateSessionParticipantCount(streamSessionID, count)
}

// ──────────────────────────────────────────────
// SQLAnalyticsRepository
// ──────────────────────────────────────────────

type SQLAnalyticsRepository struct{ db *sql.DB }

func NewSQLAnalyticsRepository(db *sql.DB) *SQLAnalyticsRepository {
	return &SQLAnalyticsRepository{db: db}
}

func (r *SQLAnalyticsRepository) RecordParticipantEvent(streamSessionID, participantDID, eventType string, geohashPrefix *string) error {
	if eventType != "join" && eventType != "leave" {
		return errors.New("event_type must be 'join' or 'leave'")
	}
	id := uuid.New().String()
	_, err := r.db.Exec(`INSERT INTO stream_participant_events(id,stream_session_id,participant_did,event_type,geohash_prefix,occurred_at)
		VALUES($1::uuid,$2::uuid,$3,$4,$5,NOW())`,
		id, streamSessionID, participantDID, eventType, nullString(geohashPrefix))
	if err != nil {
		return fmt.Errorf("record participant event: %w", err)
	}
	return nil
}

const participantEventColumns = `id::text,stream_session_id::text,participant_did,event_type,geohash_prefix,occurred_at`

func scanParticipantEvent(row rowScanner) (*ParticipantEvent, error) {
	var e ParticipantEvent
	var geohash sql.NullString
	err := row.Scan(&e.ID, &e.StreamSessionID, &e.ParticipantDID, &e.EventType, &geohash, &e.OccurredAt)
	if err != nil {
		return nil, err
	}
	if geohash.Valid && geohash.String != "" {
		e.GeohashPrefix = &geohash.String
	}
	return &e, nil
}

func (r *SQLAnalyticsRepository) GetParticipantEvents(streamSessionID string) ([]*ParticipantEvent, error) {
	rows, err := r.db.Query(`SELECT `+participantEventColumns+` FROM stream_participant_events WHERE stream_session_id=$1::uuid ORDER BY occurred_at`, streamSessionID)
	if err != nil {
		return nil, fmt.Errorf("get participant events: %w", err)
	}
	defer rows.Close()
	var result []*ParticipantEvent
	for rows.Next() {
		e, err := scanParticipantEvent(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, e)
	}
	return result, rows.Err()
}

func (r *SQLAnalyticsRepository) ComputeAnalytics(streamSessionID string) (*Analytics, error) {
	// Verify stream session exists and get timing info
	var sessionStartedAt time.Time
	var sessionEndedAt sql.NullTime
	err := r.db.QueryRow(`SELECT started_at,ended_at FROM stream_sessions WHERE id=$1::uuid`, streamSessionID).Scan(&sessionStartedAt, &sessionEndedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrStreamNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("compute analytics: %w", err)
	}

	// Get events
	events, err := r.GetParticipantEvents(streamSessionID)
	if err != nil {
		return nil, err
	}

	// Compute stream duration
	streamDuration := 0
	if sessionEndedAt.Valid {
		streamDuration = int(sessionEndedAt.Time.Sub(sessionStartedAt).Seconds())
	}

	// Compute engagement lag
	var engagementLag *int
	for _, event := range events {
		if event.EventType == "join" {
			lag := int(event.OccurredAt.Sub(sessionStartedAt).Seconds())
			if lag < 0 {
				lag = 0
			}
			engagementLag = &lag
			break
		}
	}

	// Track concurrent listeners and unique participants
	concurrent := 0
	peakConcurrent := 0
	uniqueParticipants := make(map[string]bool)
	participantJoinTimes := make(map[string]time.Time)
	var listenDurations []float64
	geoParticipants := make(map[string]map[string]bool)
	totalJoins := 0

	for _, event := range events {
		if event.EventType == "join" {
			totalJoins++
			if _, alreadyJoined := participantJoinTimes[event.ParticipantDID]; !alreadyJoined {
				concurrent++
				if concurrent > peakConcurrent {
					peakConcurrent = concurrent
				}
			}
			uniqueParticipants[event.ParticipantDID] = true
			participantJoinTimes[event.ParticipantDID] = event.OccurredAt
			if event.GeohashPrefix != nil && *event.GeohashPrefix != "" {
				prefix := *event.GeohashPrefix
				if geoParticipants[prefix] == nil {
					geoParticipants[prefix] = make(map[string]bool)
				}
				geoParticipants[prefix][event.ParticipantDID] = true
			}
		} else if event.EventType == "leave" {
			concurrent--
			if concurrent < 0 {
				concurrent = 0
			}
			if joinTime, ok := participantJoinTimes[event.ParticipantDID]; ok {
				duration := event.OccurredAt.Sub(joinTime).Seconds()
				if duration > 0 {
					listenDurations = append(listenDurations, duration)
				}
				delete(participantJoinTimes, event.ParticipantDID)
			}
		}
	}

	// Geo distribution
	geoDistribution := make(map[string]int)
	for prefix, participants := range geoParticipants {
		geoDistribution[prefix] = len(participants)
	}

	// Retention metrics
	var avgDuration *float64
	var medianDuration *float64
	if len(listenDurations) > 0 {
		sum := 0.0
		for _, d := range listenDurations {
			sum += d
		}
		avg := sum / float64(len(listenDurations))
		avgDuration = &avg
	}

	geoJSON, err := json.Marshal(geoDistribution)
	if err != nil {
		return nil, fmt.Errorf("marshal geo distribution: %w", err)
	}

	now := time.Now()
	id := uuid.New().String()

	_, err = r.db.Exec(`INSERT INTO stream_analytics(id,stream_session_id,peak_concurrent_listeners,total_unique_participants,total_join_attempts,stream_duration_seconds,engagement_lag_seconds,avg_listen_duration_seconds,median_listen_duration_seconds,geographic_distribution,computed_at)
		VALUES($1::uuid,$2::uuid,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT(stream_session_id) DO UPDATE SET peak_concurrent_listeners=EXCLUDED.peak_concurrent_listeners,total_unique_participants=EXCLUDED.total_unique_participants,total_join_attempts=EXCLUDED.total_join_attempts,stream_duration_seconds=EXCLUDED.stream_duration_seconds,engagement_lag_seconds=EXCLUDED.engagement_lag_seconds,avg_listen_duration_seconds=EXCLUDED.avg_listen_duration_seconds,median_listen_duration_seconds=EXCLUDED.median_listen_duration_seconds,geographic_distribution=EXCLUDED.geographic_distribution,computed_at=EXCLUDED.computed_at`,
		id, streamSessionID, peakConcurrent, len(uniqueParticipants), totalJoins, streamDuration, engagementLag, avgDuration, medianDuration, geoJSON, now)
	if err != nil {
		return nil, fmt.Errorf("store analytics: %w", err)
	}

	analytics := &Analytics{
		ID:                          id,
		StreamSessionID:             streamSessionID,
		PeakConcurrentListeners:     peakConcurrent,
		TotalUniqueParticipants:     len(uniqueParticipants),
		TotalJoinAttempts:           totalJoins,
		StreamDurationSeconds:       streamDuration,
		EngagementLagSeconds:        engagementLag,
		AvgListenDurationSeconds:    avgDuration,
		MedianListenDurationSeconds: medianDuration,
		GeographicDistribution:      geoDistribution,
		ComputedAt:                  now,
	}
	return analytics, nil
}

func (r *SQLAnalyticsRepository) GetAnalytics(streamSessionID string) (*Analytics, error) {
	var a Analytics
	var engageLag sql.NullInt64
	var avgDur, medianDur sql.NullFloat64
	var geoBytes []byte
	err := r.db.QueryRow(`SELECT id::text,stream_session_id::text,peak_concurrent_listeners,total_unique_participants,total_join_attempts,stream_duration_seconds,engagement_lag_seconds,avg_listen_duration_seconds,median_listen_duration_seconds,geographic_distribution,computed_at FROM stream_analytics WHERE stream_session_id=$1::uuid`, streamSessionID).Scan(
		&a.ID, &a.StreamSessionID, &a.PeakConcurrentListeners, &a.TotalUniqueParticipants, &a.TotalJoinAttempts, &a.StreamDurationSeconds,
		&engageLag, &avgDur, &medianDur, &geoBytes, &a.ComputedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrAnalyticsNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get analytics: %w", err)
	}
	if engageLag.Valid {
		v := int(engageLag.Int64)
		a.EngagementLagSeconds = &v
	}
	if avgDur.Valid {
		a.AvgListenDurationSeconds = &avgDur.Float64
	}
	if medianDur.Valid {
		a.MedianListenDurationSeconds = &medianDur.Float64
	}
	a.GeographicDistribution = make(map[string]int)
	if len(geoBytes) > 0 {
		json.Unmarshal(geoBytes, &a.GeographicDistribution)
	}
	return &a, nil
}

func isUniqueViolation(err error) bool {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23505" {
		return true
	}
	return false
}

// Compile-time interface checks
var _ SessionRepository = (*SQLSessionRepository)(nil)
var _ ParticipantRepository = (*SQLParticipantRepository)(nil)
var _ AnalyticsRepository = (*SQLAnalyticsRepository)(nil)
