package scene

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
)

// SQLRepositories groups the durable adapters for the scene bounded context.
type SQLRepositories struct {
	Scenes *SQLSceneRepository
	Events *SQLEventRepository
	RSVPs  *SQLRSVPRepository
}

func NewSQLRepositories(database *sql.DB) SQLRepositories {
	return SQLRepositories{NewSQLSceneRepository(database), NewSQLEventRepository(database), NewSQLRSVPRepository(database)}
}

type SQLSceneRepository struct{ db *sql.DB }
type SQLEventRepository struct{ db *sql.DB }
type SQLRSVPRepository struct{ db *sql.DB }

func NewSQLSceneRepository(database *sql.DB) *SQLSceneRepository {
	return &SQLSceneRepository{db: database}
}
func NewSQLEventRepository(database *sql.DB) *SQLEventRepository {
	return &SQLEventRepository{db: database}
}
func NewSQLRSVPRepository(database *sql.DB) *SQLRSVPRepository {
	return &SQLRSVPRepository{db: database}
}

const sceneColumns = `id::text,name,COALESCE(description,''),owner_did,allow_precise,
CASE WHEN allow_precise THEN ST_Y(precise_point::geometry) END,
CASE WHEN allow_precise THEN ST_X(precise_point::geometry) END,
coarse_geohash,COALESCE(tags,'{}'),COALESCE(visibility,'public'),COALESCE(palette,'{}'::jsonb),
owner_user_id::text,connected_account_id,COALESCE(connected_account_status,'pending'),account_onboarded_at,
COALESCE(moderation_status,'visible'),moderation_reason,moderated_by,moderation_timestamp,
created_at,updated_at,deleted_at,record_did,record_rkey,version`

type rowScanner interface{ Scan(...any) error }

func scanScene(row rowScanner) (*Scene, error) {
	var value Scene
	var lat, lng sql.NullFloat64
	var description, ownerUserID, connectedID, moderationReason, moderatedBy, recordDID, recordRKey sql.NullString
	var accountOnboarded, moderatedAt, createdAt, updatedAt, deletedAt sql.NullTime
	var tags pq.StringArray
	var paletteBytes []byte
	err := row.Scan(&value.ID, &value.Name, &description, &value.OwnerDID, &value.AllowPrecise, &lat, &lng,
		&value.CoarseGeohash, &tags, &value.Visibility, &paletteBytes, &ownerUserID, &connectedID,
		&value.ConnectedAccountStatus, &accountOnboarded, &value.ModerationStatus, &moderationReason,
		&moderatedBy, &moderatedAt, &createdAt, &updatedAt, &deletedAt, &recordDID, &recordRKey, &value.Version)
	if err != nil {
		return nil, err
	}
	value.Description = description.String
	value.Tags = []string(tags)
	if len(paletteBytes) > 0 && string(paletteBytes) != "{}" {
		var palette Palette
		if json.Unmarshal(paletteBytes, &palette) == nil {
			value.Palette = &palette
		}
	}
	if lat.Valid && lng.Valid {
		value.PrecisePoint = &Point{Lat: lat.Float64, Lng: lng.Float64}
	}
	value.OwnerUserID = stringPointer(ownerUserID)
	value.ConnectedAccountID = stringPointer(connectedID)
	value.ModerationReason = stringPointer(moderationReason)
	value.ModeratedBy = stringPointer(moderatedBy)
	value.AccountOnboardedAt = timePointer(accountOnboarded)
	value.ModerationTimestamp = timePointer(moderatedAt)
	value.CreatedAt = timePointer(createdAt)
	value.UpdatedAt = timePointer(updatedAt)
	value.DeletedAt = timePointer(deletedAt)
	value.RecordDID = stringPointer(recordDID)
	value.RecordRKey = stringPointer(recordRKey)
	return &value, nil
}

func stringPointer(value sql.NullString) *string {
	if !value.Valid {
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

func locationSQL(point *Point, allowed bool) (any, any) {
	if !allowed || point == nil {
		return nil, nil
	}
	return point.Lng, point.Lat
}

func (r *SQLSceneRepository) Insert(value *Scene) error {
	copy := *value
	copy.EnforceLocationConsent()
	lng, lat := locationSQL(copy.PrecisePoint, copy.AllowPrecise)
	palette, _ := json.Marshal(copy.Palette)
	if copy.Version == 0 {
		copy.Version = 1
	}
	row := r.db.QueryRow(`INSERT INTO scenes
		(id,name,description,owner_did,allow_precise,precise_point,coarse_geohash,tags,visibility,palette,owner_user_id,
		 connected_account_id,connected_account_status,account_onboarded_at,moderation_status,moderation_reason,moderated_by,
		 moderation_timestamp,record_did,record_rkey,version,publication_status)
		VALUES($1,$2,$3,$4,$5,CASE WHEN $6::float8 IS NULL THEN NULL ELSE ST_SetSRID(ST_MakePoint($6,$7),4326)::geography END,
		$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23)
		RETURNING created_at,updated_at`, copy.ID, copy.Name, copy.Description, copy.OwnerDID, copy.AllowPrecise, lng, lat,
		copy.CoarseGeohash, pq.Array(copy.Tags), defaultString(copy.Visibility, VisibilityPublic), palette, nullString(copy.OwnerUserID),
		nullString(copy.ConnectedAccountID), defaultString(copy.ConnectedAccountStatus, "pending"), nullTime(copy.AccountOnboardedAt),
		defaultString(copy.ModerationStatus, "visible"), nullString(copy.ModerationReason), nullString(copy.ModeratedBy),
		nullTime(copy.ModerationTimestamp), nullString(copy.RecordDID), nullString(copy.RecordRKey), copy.Version, defaultString(copy.PublicationStatus, "published"))
	if err := row.Scan(&copy.CreatedAt, &copy.UpdatedAt); err != nil {
		return mapSceneError(err)
	}
	*value = copy
	return nil
}

func (r *SQLSceneRepository) Update(value *Scene) error {
	copy := *value
	copy.EnforceLocationConsent()
	lng, lat := locationSQL(copy.PrecisePoint, copy.AllowPrecise)
	palette, _ := json.Marshal(copy.Palette)
	result := r.db.QueryRow(`UPDATE scenes SET name=$2,description=$3,owner_did=$4,allow_precise=$5,
		precise_point=CASE WHEN $6::float8 IS NULL THEN NULL ELSE ST_SetSRID(ST_MakePoint($6,$7),4326)::geography END,
		coarse_geohash=$8,tags=$9,visibility=$10,palette=$11,connected_account_id=$12,
		connected_account_status=$13,account_onboarded_at=$14,updated_at=NOW(),version=version+1
		WHERE id=$1 AND deleted_at IS NULL AND version=$15 RETURNING version,updated_at`, copy.ID, copy.Name, copy.Description,
		copy.OwnerDID, copy.AllowPrecise, lng, lat, copy.CoarseGeohash, pq.Array(copy.Tags), defaultString(copy.Visibility, VisibilityPublic),
		palette, nullString(copy.ConnectedAccountID), defaultString(copy.ConnectedAccountStatus, "pending"), nullTime(copy.AccountOnboardedAt), copy.Version)
	if err := result.Scan(&copy.Version, &copy.UpdatedAt); errors.Is(err, sql.ErrNoRows) {
		return ErrVersionConflict
	} else if err != nil {
		return mapSceneError(err)
	}
	*value = copy
	return nil
}

func (r *SQLSceneRepository) Upsert(value *Scene) (*UpsertResult, error) {
	if value.RecordDID != nil && value.RecordRKey != nil {
		existing, err := r.GetByRecordKey(*value.RecordDID, *value.RecordRKey)
		if err == nil {
			value.ID = existing.ID
			value.Version = existing.Version
			if err = r.Update(value); err != nil {
				return nil, err
			}
			return &UpsertResult{ID: value.ID}, nil
		}
		if !errors.Is(err, ErrSceneNotFound) {
			return nil, err
		}
	}
	if err := r.Insert(value); err != nil {
		return nil, err
	}
	return &UpsertResult{Inserted: true, ID: value.ID}, nil
}

func (r *SQLSceneRepository) GetByID(id string) (*Scene, error) {
	value, err := scanScene(r.db.QueryRow(`SELECT `+sceneColumns+` FROM scenes WHERE id=$1 AND deleted_at IS NULL`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrSceneNotFound
	}
	return value, err
}
func (r *SQLSceneRepository) GetByRecordKey(did, rkey string) (*Scene, error) {
	value, err := scanScene(r.db.QueryRow(`SELECT `+sceneColumns+` FROM scenes WHERE record_did=$1 AND record_rkey=$2 AND deleted_at IS NULL`, did, rkey))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrSceneNotFound
	}
	return value, err
}
func (r *SQLSceneRepository) Delete(id string) error {
	result, err := r.db.Exec(`UPDATE scenes SET deleted_at=NOW(),updated_at=NOW() WHERE id=$1 AND deleted_at IS NULL`, id)
	if err != nil {
		return err
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		return ErrSceneNotFound
	}
	return nil
}
func (r *SQLSceneRepository) ExistsByOwnerAndName(owner, name, exclude string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM scenes WHERE owner_did=$1 AND lower(name)=lower($2) AND deleted_at IS NULL AND ($3='' OR id<>NULLIF($3,'')::uuid))`, owner, name, exclude).Scan(&exists)
	return exists, err
}
func (r *SQLSceneRepository) ListByOwner(owner string) ([]*Scene, error) {
	return r.list(`owner_did=$1`, owner)
}
func (r *SQLSceneRepository) ListByConnectedAccountID(id string) ([]*Scene, error) {
	return r.list(`connected_account_id=$1`, id)
}
func (r *SQLSceneRepository) list(predicate string, arg any) ([]*Scene, error) {
	rows, err := r.db.Query(`SELECT `+sceneColumns+` FROM scenes WHERE deleted_at IS NULL AND `+predicate+` ORDER BY created_at DESC,id`, arg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*Scene
	for rows.Next() {
		v, e := scanScene(rows)
		if e != nil {
			return nil, e
		}
		result = append(result, v)
	}
	return result, rows.Err()
}
func (r *SQLSceneRepository) SearchScenes(opts SceneSearchOptions) ([]*Scene, string, error) {
	limit := opts.Limit
	if limit <= 0 || limit > 50 {
		limit = 50
	}
	pattern := "%" + strings.TrimSpace(opts.Query) + "%"
	rows, err := r.db.Query(`SELECT `+sceneColumns+` FROM scenes WHERE deleted_at IS NULL AND COALESCE(moderation_status,'visible') IN ('visible','flagged') AND visibility='public'
		AND ($1='' OR name ILIKE $2 OR COALESCE(description,'') ILIKE $2) AND ($3::text[]='{}' OR tags && $3)
		AND (precise_point IS NULL OR ST_Intersects(precise_point,ST_MakeEnvelope($4,$5,$6,$7,4326)::geography))
		ORDER BY updated_at DESC,id LIMIT $8 OFFSET $9`, strings.TrimSpace(opts.Query), pattern, pq.Array(opts.Genres), opts.MinLng, opts.MinLat, opts.MaxLng, opts.MaxLat, limit, opts.Offset)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	var result []*Scene
	for rows.Next() {
		v, e := scanScene(rows)
		if e != nil {
			return nil, "", e
		}
		result = append(result, v)
	}
	return result, "", rows.Err()
}

const eventColumns = `id::text,scene_id::text,title,COALESCE(description,''),allow_precise,
CASE WHEN $1::boolean AND location_access='public' AND allow_precise THEN ST_Y(precise_point::geometry) END,
CASE WHEN $1::boolean AND location_access='public' AND allow_precise THEN ST_X(precise_point::geometry) END,
coarse_geohash,place_id::text,venue_id::text,COALESCE(kind,'show'),COALESCE(location_access,'public'),COALESCE(tags,'{}'),
COALESCE(status,'scheduled'),starts_at,ends_at,created_at,updated_at,deleted_at,cancelled_at,cancellation_reason,
record_did,record_rkey,stream_session_id::text,version`

func scanEvent(row rowScanner) (*Event, error) {
	var v Event
	var lat, lng sql.NullFloat64
	var desc, place, venue, cancel, rdid, rkey, stream sql.NullString
	var ends, created, updated, deleted, cancelled sql.NullTime
	var tags pq.StringArray
	err := row.Scan(&v.ID, &v.SceneID, &v.Title, &desc, &v.AllowPrecise, &lat, &lng, &v.CoarseGeohash, &place, &venue, &v.Kind, &v.LocationAccess, &tags, &v.Status, &v.StartsAt, &ends, &created, &updated, &deleted, &cancelled, &cancel, &rdid, &rkey, &stream, &v.Version)
	if err != nil {
		return nil, err
	}
	v.Description = desc.String
	v.Tags = []string(tags)
	if lat.Valid && lng.Valid {
		v.PrecisePoint = &Point{Lat: lat.Float64, Lng: lng.Float64}
	}
	v.PlaceID = stringPointer(place)
	v.VenueID = stringPointer(venue)
	v.EndsAt = timePointer(ends)
	v.CreatedAt = timePointer(created)
	v.UpdatedAt = timePointer(updated)
	v.DeletedAt = timePointer(deleted)
	v.CancelledAt = timePointer(cancelled)
	v.CancellationReason = stringPointer(cancel)
	v.RecordDID = stringPointer(rdid)
	v.RecordRKey = stringPointer(rkey)
	v.StreamSessionID = stringPointer(stream)
	return &v, nil
}

func (r *SQLEventRepository) Insert(value *Event) error {
	copy := *value
	copy.EnforceLocationConsent()
	lng, lat := locationSQL(copy.PrecisePoint, copy.AllowPrecise)
	if copy.Version == 0 {
		copy.Version = 1
	}
	row := r.db.QueryRow(`INSERT INTO events(id,scene_id,title,description,allow_precise,precise_point,coarse_geohash,place_id,venue_id,kind,location_access,tags,status,starts_at,ends_at,record_did,record_rkey,stream_session_id,version,publication_status)
	VALUES($1,$2,$3,$4,$5,CASE WHEN $6::float8 IS NULL THEN NULL ELSE ST_SetSRID(ST_MakePoint($6,$7),4326)::geography END,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21) RETURNING created_at,updated_at`, copy.ID, copy.SceneID, copy.Title, copy.Description, copy.AllowPrecise, lng, lat, copy.CoarseGeohash, nullString(copy.PlaceID), nullString(copy.VenueID), defaultString(copy.Kind, "show"), defaultString(copy.LocationAccess, "public"), pq.Array(copy.Tags), defaultString(copy.Status, "scheduled"), copy.StartsAt, nullTime(copy.EndsAt), nullString(copy.RecordDID), nullString(copy.RecordRKey), nullString(copy.StreamSessionID), copy.Version, defaultString(copy.PublicationStatus, "published"))
	if err := row.Scan(&copy.CreatedAt, &copy.UpdatedAt); err != nil {
		return mapEventError(err)
	}
	*value = copy
	return nil
}
func (r *SQLEventRepository) Update(value *Event) error {
	copy := *value
	copy.EnforceLocationConsent()
	lng, lat := locationSQL(copy.PrecisePoint, copy.AllowPrecise)
	row := r.db.QueryRow(`UPDATE events SET title=$2,description=$3,allow_precise=$4,precise_point=CASE WHEN $5::float8 IS NULL THEN NULL ELSE ST_SetSRID(ST_MakePoint($5,$6),4326)::geography END,coarse_geohash=$7,place_id=$8,venue_id=$9,kind=$10,location_access=$11,tags=$12,status=$13,starts_at=$14,ends_at=$15,updated_at=NOW(),version=version+1 WHERE id=$1 AND deleted_at IS NULL AND version=$16 RETURNING version,updated_at`, copy.ID, copy.Title, copy.Description, copy.AllowPrecise, lng, lat, copy.CoarseGeohash, nullString(copy.PlaceID), nullString(copy.VenueID), defaultString(copy.Kind, "show"), defaultString(copy.LocationAccess, "public"), pq.Array(copy.Tags), defaultString(copy.Status, "scheduled"), copy.StartsAt, nullTime(copy.EndsAt), copy.Version)
	if err := row.Scan(&copy.Version, &copy.UpdatedAt); errors.Is(err, sql.ErrNoRows) {
		return ErrVersionConflict
	} else if err != nil {
		return mapEventError(err)
	}
	*value = copy
	return nil
}
func (r *SQLEventRepository) Upsert(value *Event) (*UpsertResult, error) {
	if value.RecordDID != nil && value.RecordRKey != nil {
		old, err := r.GetByRecordKey(*value.RecordDID, *value.RecordRKey)
		if err == nil {
			value.ID = old.ID
			value.Version = old.Version
			if err = r.Update(value); err != nil {
				return nil, err
			}
			return &UpsertResult{ID: value.ID}, nil
		}
		if !errors.Is(err, ErrEventNotFound) {
			return nil, err
		}
	}
	if err := r.Insert(value); err != nil {
		return nil, err
	}
	return &UpsertResult{Inserted: true, ID: value.ID}, nil
}
func (r *SQLEventRepository) GetByID(id string) (*Event, error) {
	v, err := scanEvent(r.db.QueryRow(`SELECT `+eventColumns+` FROM events WHERE id=$2 AND deleted_at IS NULL`, false, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrEventNotFound
	}
	return v, err
}
func (r *SQLEventRepository) GetByRecordKey(did, key string) (*Event, error) {
	v, err := scanEvent(r.db.QueryRow(`SELECT `+eventColumns+` FROM events WHERE record_did=$2 AND record_rkey=$3 AND deleted_at IS NULL`, false, did, key))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrEventNotFound
	}
	return v, err
}
func (r *SQLEventRepository) Cancel(id string, reason *string) error {
	result, err := r.db.Exec(`UPDATE events SET status='cancelled',cancelled_at=COALESCE(cancelled_at,NOW()),cancellation_reason=COALESCE($2,cancellation_reason),updated_at=NOW(),version=version+1 WHERE id=$1 AND deleted_at IS NULL AND cancelled_at IS NULL`, id, nullString(reason))
	if err != nil {
		return err
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		var exists bool
		if e := r.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM events WHERE id=$1 AND deleted_at IS NULL)`, id).Scan(&exists); e != nil {
			return e
		}
		if !exists {
			return ErrEventNotFound
		}
	}
	return nil
}
func (r *SQLEventRepository) SearchByBboxAndTime(a, b, c, d float64, from, to time.Time, limit int, cursor string) ([]*Event, string, error) {
	return r.SearchEvents(EventSearchOptions{MinLng: a, MinLat: b, MaxLng: c, MaxLat: d, From: from, To: to, Limit: limit, Cursor: cursor})
}
func (r *SQLEventRepository) SearchEvents(opts EventSearchOptions) ([]*Event, string, error) {
	limit := opts.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	pattern := "%" + strings.TrimSpace(opts.Query) + "%"
	rows, err := r.db.Query(`SELECT `+eventColumns+` FROM events WHERE deleted_at IS NULL AND publication_status='published' AND cancelled_at IS NULL AND ($2::timestamptz='0001-01-01'::timestamptz OR starts_at >= $2) AND ($3::timestamptz='0001-01-01'::timestamptz OR starts_at <= $3) AND ($4='' OR title ILIKE $5 OR COALESCE(description,'') ILIKE $5) AND ($6='' OR scene_id=NULLIF($6,'')::uuid) AND ($7::uuid[]='{}' OR scene_id=ANY($7)) ORDER BY starts_at,id LIMIT $8`, true, opts.From, opts.To, strings.TrimSpace(opts.Query), pattern, opts.SceneID, pq.Array(opts.SceneIDs), limit*4)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	result := make([]*Event, 0, limit)
	for rows.Next() {
		v, e := scanEvent(rows)
		if e != nil {
			return nil, "", e
		}
		inside := v.PrecisePoint != nil && v.PrecisePoint.Lng >= opts.MinLng && v.PrecisePoint.Lng <= opts.MaxLng && v.PrecisePoint.Lat >= opts.MinLat && v.PrecisePoint.Lat <= opts.MaxLat
		if !inside && v.CoarseGeohash != "" {
			inside = CoarseGeohashIntersectsBBox(v.CoarseGeohash, opts.MinLng, opts.MinLat, opts.MaxLng, opts.MaxLat)
		}
		if inside || opts.MinLng == opts.MaxLng {
			result = append(result, v)
		}
		if len(result) == limit {
			break
		}
	}
	return result, "", rows.Err()
}

func (r *SQLRSVPRepository) Upsert(v *RSVP) error {
	row := r.db.QueryRow(`INSERT INTO event_rsvps(event_id,user_id,status) VALUES($1,$2,$3) ON CONFLICT(event_id,user_id) DO UPDATE SET status=EXCLUDED.status,updated_at=NOW() RETURNING created_at,updated_at`, v.EventID, v.UserID, v.Status)
	return row.Scan(&v.CreatedAt, &v.UpdatedAt)
}
func (r *SQLRSVPRepository) Delete(eventID, userID string) error {
	result, err := r.db.Exec(`DELETE FROM event_rsvps WHERE event_id=$1 AND user_id=$2`, eventID, userID)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrRSVPNotFound
	}
	return nil
}
func (r *SQLRSVPRepository) GetByEventAndUser(eventID, userID string) (*RSVP, error) {
	v := &RSVP{}
	err := r.db.QueryRow(`SELECT event_id::text,user_id,status,created_at,updated_at FROM event_rsvps WHERE event_id=$1 AND user_id=$2`, eventID, userID).Scan(&v.EventID, &v.UserID, &v.Status, &v.CreatedAt, &v.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrRSVPNotFound
	}
	return v, err
}
func (r *SQLRSVPRepository) GetCountsByEvent(eventID string) (*RSVPCounts, error) {
	v := &RSVPCounts{}
	err := r.db.QueryRow(`SELECT count(*) FILTER(WHERE status='going'),count(*) FILTER(WHERE status='maybe') FROM event_rsvps WHERE event_id=$1`, eventID).Scan(&v.Going, &v.Maybe)
	return v, err
}
func (r *SQLRSVPRepository) GetCountsForEvents(ids []string) (map[string]*RSVPCounts, error) {
	result := make(map[string]*RSVPCounts, len(ids))
	for _, id := range ids {
		result[id] = &RSVPCounts{}
	}
	if len(ids) == 0 {
		return result, nil
	}
	rows, err := r.db.Query(`SELECT event_id::text,count(*) FILTER(WHERE status='going'),count(*) FILTER(WHERE status='maybe') FROM event_rsvps WHERE event_id=ANY($1) GROUP BY event_id`, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		v := &RSVPCounts{}
		if err = rows.Scan(&id, &v.Going, &v.Maybe); err != nil {
			return nil, err
		}
		result[id] = v
	}
	return result, rows.Err()
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
func mapSceneError(err error) error {
	if pqerr := new(pq.Error); errors.As(err, &pqerr) && pqerr.Code == "23505" {
		return ErrDuplicateSceneName
	}
	return fmt.Errorf("scene repository: %w", err)
}
func mapEventError(err error) error { return fmt.Errorf("event repository: %w", err) }

var _ SceneRepository = (*SQLSceneRepository)(nil)
var _ EventRepository = (*SQLEventRepository)(nil)
var _ RSVPRepository = (*SQLRSVPRepository)(nil)
