package touring

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"github.com/lib/pq"
)

// SQLRepository persists touring entities and their append-only provenance.
type SQLRepository struct{ db *sql.DB }

func NewSQLRepository(database *sql.DB) *SQLRepository { return &SQLRepository{db: database} }

func (r *SQLRepository) StorePlace(v Place) error {
	if err := v.Validate(); err != nil || strings.TrimSpace(v.ID) == "" {
		return ErrInvalidPlace
	}
	if v.Version == 0 {
		v.Version = 1
	}
	_, err := r.db.Exec(`INSERT INTO places(id,canonical_name,admin_region,country_code,timezone,coarse_geohash,version,created_by_user_id,publication_status)
		VALUES($1,$2,NULLIF($3,''),upper($4),$5,$6,$7,NULLIF($8,'')::uuid,$9)`, v.ID, v.CanonicalName, v.AdminRegion, v.CountryCode, v.Timezone, v.CoarseGeohash, v.Version, v.CreatedByUserID, defaultPublicationStatus(v.PublicationStatus))
	return mapTouringError(err, ErrInvalidPlace)
}

func (r *SQLRepository) StoreVenue(v Venue) error {
	v.EnforceLocationConsent()
	if err := v.Validate(); err != nil {
		return err
	}
	if v.Version == 0 {
		v.Version = 1
	}
	var lat, lng any
	if v.PrecisePoint != nil {
		lat, lng = v.PrecisePoint.Lat, v.PrecisePoint.Lng
	}
	_, err := r.db.Exec(`INSERT INTO venues(id,place_id,canonical_name,allow_precise,precise_point,coarse_geohash,version,publication_status)
		VALUES($1,$2,$3,$4,CASE WHEN $5::double precision IS NULL THEN NULL ELSE ST_SetSRID(ST_MakePoint($6,$5),4326)::geography END,$7,$8,$9)`,
		v.ID, v.PlaceID, v.CanonicalName, v.AllowPrecise, lat, lng, v.CoarseGeohash, v.Version, defaultPublicationStatus(v.PublicationStatus))
	return mapTouringError(err, ErrInvalidPlace)
}

func (r *SQLRepository) StoreProfile(v Profile) error {
	if err := v.Validate(); err != nil || strings.TrimSpace(v.ID) == "" {
		return ErrInvalidProfile
	}
	if v.Version == 0 {
		v.Version = 1
	}
	_, err := r.db.Exec(`INSERT INTO profiles(id,kind,canonical_name,visibility,version,created_by_user_id,publication_status) VALUES($1,$2,$3,$4,$5,NULLIF($6,'')::uuid,$7)`, v.ID, v.Kind, v.CanonicalName, v.Visibility, v.Version, v.CreatedByUserID, defaultPublicationStatus(v.PublicationStatus))
	return mapTouringError(err, ErrInvalidProfile)
}
func (r *SQLRepository) StoreAct(v Act) error {
	if strings.TrimSpace(v.ID) == "" || strings.TrimSpace(v.ProfileID) == "" {
		return ErrInvalidProfile
	}
	_, err := r.db.Exec(`INSERT INTO acts(id,profile_id,publication_status) VALUES($1,$2,$3)`, v.ID, v.ProfileID, defaultPublicationStatus(v.PublicationStatus))
	return mapTouringError(err, ErrInvalidProfile)
}
func (r *SQLRepository) UpdatePlace(v *Place) error {
	if err := v.Validate(); err != nil {
		return err
	}
	return scanVersion(r.db.QueryRow(`UPDATE places SET canonical_name=$2,admin_region=NULLIF($3,''),country_code=upper($4),timezone=$5,coarse_geohash=$6,updated_at=NOW(),version=version+1 WHERE id=$1 AND version=$7 RETURNING version`, v.ID, v.CanonicalName, v.AdminRegion, v.CountryCode, v.Timezone, v.CoarseGeohash, v.Version), &v.Version)
}
func (r *SQLRepository) UpdateVenue(v *Venue) error {
	v.EnforceLocationConsent()
	if err := v.Validate(); err != nil {
		return err
	}
	var lat, lng any
	if v.PrecisePoint != nil {
		lat, lng = v.PrecisePoint.Lat, v.PrecisePoint.Lng
	}
	return scanVersion(r.db.QueryRow(`UPDATE venues SET place_id=$2,canonical_name=$3,allow_precise=$4,
		precise_point=CASE WHEN $5::double precision IS NULL THEN NULL ELSE ST_SetSRID(ST_MakePoint($6,$5),4326)::geography END,
		coarse_geohash=$7,updated_at=NOW(),version=version+1 WHERE id=$1 AND version=$8 RETURNING version`,
		v.ID, v.PlaceID, v.CanonicalName, v.AllowPrecise, lat, lng, v.CoarseGeohash, v.Version), &v.Version)
}
func (r *SQLRepository) UpdateProfile(v *Profile) error {
	if err := v.Validate(); err != nil {
		return err
	}
	return scanVersion(r.db.QueryRow(`UPDATE profiles SET kind=$2,canonical_name=$3,visibility=$4,updated_at=NOW(),version=version+1 WHERE id=$1 AND version=$5 RETURNING version`, v.ID, v.Kind, v.CanonicalName, v.Visibility, v.Version), &v.Version)
}
func (r *SQLRepository) UpdateTour(v *Tour) error {
	if err := v.Validate(); err != nil {
		return err
	}
	return scanVersion(r.db.QueryRow(`UPDATE tours SET primary_act_id=$2,title=$3,status=$4,starts_on=$5,ends_on=$6,updated_at=NOW(),version=version+1 WHERE id=$1 AND version=$7 RETURNING version`, v.ID, v.PrimaryActID, v.Title, v.Status, v.StartsOn, v.EndsOn, v.Version), &v.Version)
}
func (r *SQLRepository) UpdateAppearance(v *Appearance) error {
	if err := v.Validate(); err != nil {
		return err
	}
	return scanVersion(r.db.QueryRow(`UPDATE appearances SET event_id=$2,act_id=$3,tour_id=$4,role=$5,stage_name=NULLIF($6,''),starts_at=$7,ends_at=$8,status=$9,updated_at=NOW(),version=version+1 WHERE id=$1 AND version=$10 RETURNING version`, v.ID, v.EventID, v.ActID, v.TourID, v.Role, v.StageName, v.StartsAt, v.EndsAt, v.Status, v.Version), &v.Version)
}
func scanVersion(row *sql.Row, version *int64) error {
	err := row.Scan(version)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrVersionConflict
	}
	return err
}
func (r *SQLRepository) AddHomeTerritory(v HomeTerritory) error {
	if err := v.Validate(); err != nil {
		return err
	}
	_, err := r.db.Exec(`INSERT INTO act_home_territories(id,act_id,place_id,visibility,valid_from,valid_to,asserted_by_did) VALUES($1,$2,$3,$4,$5,$6,$7)`, v.ID, v.ActID, v.PlaceID, v.Visibility, v.ValidFrom, v.ValidTo, v.AssertedByDID)
	return mapTouringError(err, ErrInvalidHomeTerritory)
}
func (r *SQLRepository) GetPlace(id string) (Place, error) {
	var v Place
	err := r.db.QueryRow(`SELECT p.id::text,p.canonical_name,COALESCE(p.admin_region,''),p.country_code,p.timezone,p.coarse_geohash,p.version,p.publication_status,
		COALESCE(m.at_uri,''),COALESCE(m.cid,''),COALESCE(m.publisher_did,''),COALESCE(l.handle,''),COALESCE(m.projection_status,'')
		FROM places p LEFT JOIN atproto_record_mappings m ON m.entity_type='place' AND m.entity_id=p.id
		LEFT JOIN atproto_oauth_links l ON l.account_did=m.publisher_did WHERE p.id=$1`, id).Scan(&v.ID, &v.CanonicalName, &v.AdminRegion, &v.CountryCode, &v.Timezone, &v.CoarseGeohash, &v.Version, &v.PublicationStatus, &v.ATURI, &v.CID, &v.PublisherDID, &v.PublisherHandle, &v.ProjectionStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return Place{}, ErrInvalidPlace
	}
	return v, err
}
func (r *SQLRepository) GetVenue(id string) (Venue, error) {
	var v Venue
	var lat, lng sql.NullFloat64
	err := r.db.QueryRow(`SELECT v.id::text,v.place_id::text,v.canonical_name,v.allow_precise,
		ST_Y(v.precise_point::geometry),ST_X(v.precise_point::geometry),v.coarse_geohash,v.version,v.publication_status,
		COALESCE(m.at_uri,''),COALESCE(m.cid,''),COALESCE(m.publisher_did,''),COALESCE(l.handle,''),COALESCE(m.projection_status,'')
		FROM venues v LEFT JOIN atproto_record_mappings m ON m.entity_type='venue' AND m.entity_id=v.id
		LEFT JOIN atproto_oauth_links l ON l.account_did=m.publisher_did WHERE v.id=$1`, id).
		Scan(&v.ID, &v.PlaceID, &v.CanonicalName, &v.AllowPrecise, &lat, &lng, &v.CoarseGeohash, &v.Version,
			&v.PublicationStatus, &v.ATURI, &v.CID, &v.PublisherDID, &v.PublisherHandle, &v.ProjectionStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return Venue{}, ErrInvalidPlace
	}
	if err == nil && lat.Valid && lng.Valid {
		v.PrecisePoint = &Point{Lat: lat.Float64, Lng: lng.Float64}
	}
	return v, err
}
func (r *SQLRepository) GetProfile(id string) (Profile, error) {
	var v Profile
	err := r.db.QueryRow(`SELECT p.id::text,p.kind,p.canonical_name,p.visibility,p.version,p.publication_status,
		COALESCE(m.at_uri,''),COALESCE(m.cid,''),COALESCE(m.publisher_did,''),COALESCE(l.handle,''),COALESCE(m.projection_status,'')
		FROM profiles p LEFT JOIN atproto_record_mappings m ON m.entity_type='profile' AND m.entity_id=p.id
		LEFT JOIN atproto_oauth_links l ON l.account_did=m.publisher_did WHERE p.id=$1`, id).Scan(&v.ID, &v.Kind, &v.CanonicalName, &v.Visibility, &v.Version, &v.PublicationStatus, &v.ATURI, &v.CID, &v.PublisherDID, &v.PublisherHandle, &v.ProjectionStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return Profile{}, ErrInvalidProfile
	}
	return v, err
}
func (r *SQLRepository) GetAct(id string) (Act, error) {
	var v Act
	err := r.db.QueryRow(`SELECT id::text,profile_id::text FROM acts WHERE id=$1`, id).Scan(&v.ID, &v.ProfileID)
	if errors.Is(err, sql.ErrNoRows) {
		return Act{}, ErrInvalidProfile
	}
	return v, err
}
func (r *SQLRepository) FindActByProfile(id string) (Act, error) {
	var v Act
	err := r.db.QueryRow(`SELECT id::text,profile_id::text FROM acts WHERE profile_id=$1`, id).Scan(&v.ID, &v.ProfileID)
	if errors.Is(err, sql.ErrNoRows) {
		return Act{}, ErrInvalidProfile
	}
	return v, err
}
func (r *SQLRepository) ListHomeTerritories(actID string) ([]HomeTerritory, error) {
	rows, err := r.db.Query(`SELECT id::text,act_id::text,place_id::text,visibility,valid_from,valid_to,asserted_by_did FROM act_home_territories WHERE act_id=$1 ORDER BY valid_from DESC,id`, actID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []HomeTerritory
	for rows.Next() {
		var v HomeTerritory
		if err = rows.Scan(&v.ID, &v.ActID, &v.PlaceID, &v.Visibility, &v.ValidFrom, &v.ValidTo, &v.AssertedByDID); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *SQLRepository) CreateTour(v Tour, addedBy string) error {
	if err := v.Validate(); err != nil {
		return err
	}
	if v.Version == 0 {
		v.Version = 1
	}
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.Exec(`INSERT INTO tours(id,primary_act_id,title,status,starts_on,ends_on,version,created_by_user_id,publication_status) VALUES($1,$2,$3,$4,$5,$6,$7,NULLIF($8,'')::uuid,$9)`, v.ID, v.PrimaryActID, v.Title, v.Status, v.StartsOn, v.EndsOn, v.Version, v.CreatedByUserID, defaultPublicationStatus(v.PublicationStatus))
	if err != nil {
		return mapTouringError(err, ErrDuplicateTour)
	}
	_, err = tx.Exec(`INSERT INTO tour_acts(tour_id,act_id,role,added_by_did) VALUES($1,$2,'primary',$3)`, v.ID, v.PrimaryActID, addedBy)
	if err != nil {
		return mapTouringError(err, ErrDuplicateTourAct)
	}
	return tx.Commit()
}
func (r *SQLRepository) GetTour(id string) (Tour, error) {
	var v Tour
	err := r.db.QueryRow(`SELECT t.id::text,t.primary_act_id::text,t.title,t.status,t.starts_on,t.ends_on,t.version,t.publication_status,
		COALESCE(m.at_uri,''),COALESCE(m.cid,''),COALESCE(m.publisher_did,''),COALESCE(l.handle,''),COALESCE(m.projection_status,'')
		FROM tours t LEFT JOIN atproto_record_mappings m ON m.entity_type='tour' AND m.entity_id=t.id
		LEFT JOIN atproto_oauth_links l ON l.account_did=m.publisher_did WHERE t.id=$1`, id).Scan(&v.ID, &v.PrimaryActID, &v.Title, &v.Status, &v.StartsOn, &v.EndsOn, &v.Version, &v.PublicationStatus, &v.ATURI, &v.CID, &v.PublisherDID, &v.PublisherHandle, &v.ProjectionStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return Tour{}, ErrTourNotFound
	}
	return v, err
}
func (r *SQLRepository) AddTourAct(v TourAct) error {
	if err := v.Validate(); err != nil {
		return err
	}
	_, err := r.db.Exec(`INSERT INTO tour_acts(tour_id,act_id,role,added_by_did) VALUES($1,$2,$3,$4)`, v.TourID, v.ActID, v.Role, v.AddedByDID)
	return mapTouringError(err, ErrDuplicateTourAct)
}
func (r *SQLRepository) ListTourActs(id string) ([]TourAct, error) {
	rows, err := r.db.Query(`SELECT tour_id::text,act_id::text,role,added_by_did FROM tour_acts WHERE tour_id=$1 ORDER BY role,act_id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TourAct
	for rows.Next() {
		var v TourAct
		if err = rows.Scan(&v.TourID, &v.ActID, &v.Role, &v.AddedByDID); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	if len(out) == 0 {
		if _, e := r.GetTour(id); e != nil {
			return nil, e
		}
	}
	return out, rows.Err()
}
func (r *SQLRepository) CreateAppearance(v Appearance) error {
	if err := v.Validate(); err != nil {
		return err
	}
	if v.Version == 0 {
		v.Version = 1
	}
	_, err := r.db.Exec(`INSERT INTO appearances(id,event_id,act_id,tour_id,role,stage_name,starts_at,ends_at,status,version,created_by_user_id,publication_status) VALUES($1,$2,$3,$4,$5,NULLIF($6,''),$7,$8,$9,$10,NULLIF($11,'')::uuid,$12)`, v.ID, v.EventID, v.ActID, v.TourID, v.Role, v.StageName, v.StartsAt, v.EndsAt, v.Status, v.Version, v.CreatedByUserID, defaultPublicationStatus(v.PublicationStatus))
	return mapTouringError(err, ErrDuplicateAppearance)
}
func scanAppearance(row interface{ Scan(...any) error }) (Appearance, error) {
	var v Appearance
	err := row.Scan(&v.ID, &v.EventID, &v.ActID, &v.TourID, &v.Role, &v.StageName, &v.StartsAt, &v.EndsAt, &v.Status, &v.Version, &v.PublicationStatus, &v.ATURI, &v.CID, &v.PublisherDID, &v.PublisherHandle, &v.ProjectionStatus)
	return v, err
}

const appearanceSelect = `SELECT id,event_id,act_id,tour_id,role,stage_name,starts_at,ends_at,status,version,publication_status,at_uri,cid,publisher_did,publisher_handle,projection_status FROM (
	SELECT a.id::text AS id,a.event_id::text AS event_id,a.act_id::text AS act_id,a.tour_id::text AS tour_id,a.role,
	COALESCE(a.stage_name,'') AS stage_name,a.starts_at,a.ends_at,a.status,a.version,a.publication_status,COALESCE(m.at_uri,'') AS at_uri,
	COALESCE(m.cid,'') AS cid,COALESCE(m.publisher_did,'') AS publisher_did,COALESCE(l.handle,'') AS publisher_handle,COALESCE(m.projection_status,'') AS projection_status
	FROM appearances a LEFT JOIN atproto_record_mappings m ON m.entity_type='appearance' AND m.entity_id=a.id
	LEFT JOIN atproto_oauth_links l ON l.account_did=m.publisher_did
) projected_appearances`

func (r *SQLRepository) GetAppearance(id string) (Appearance, error) {
	v, err := scanAppearance(r.db.QueryRow(appearanceSelect+` WHERE id=$1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Appearance{}, ErrAppearanceNotFound
	}
	return v, err
}
func (r *SQLRepository) listAppearances(predicate string, args ...any) ([]Appearance, error) {
	rows, err := r.db.Query(appearanceSelect+` WHERE `+predicate+` ORDER BY starts_at NULLS LAST,id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Appearance
	for rows.Next() {
		v, e := scanAppearance(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (r *SQLRepository) ListAppearancesForTour(id string) ([]Appearance, error) {
	out, err := r.listAppearances(`tour_id=$1`, id)
	if err == nil && len(out) == 0 {
		if _, e := r.GetTour(id); e != nil {
			return nil, e
		}
	}
	return out, err
}
func (r *SQLRepository) ListAppearances() ([]Appearance, error) { return r.listAppearances(`TRUE`) }
func (r *SQLRepository) ListAppearancesForAct(id string) ([]Appearance, error) {
	return r.listAppearances(`act_id=$1`, id)
}
func (r *SQLRepository) AddEventHost(v EventHost) error {
	if err := v.Validate(); err != nil {
		return err
	}
	_, err := r.db.Exec(`INSERT INTO event_hosts(event_id,scene_id,profile_id,role) VALUES($1,$2,$3,$4)`, v.EventID, v.SceneID, v.ProfileID, v.Role)
	return mapTouringError(err, ErrDuplicateHost)
}
func (r *SQLRepository) ListEventHosts(id string) ([]EventHost, error) {
	rows, err := r.db.Query(`SELECT event_id::text,scene_id::text,profile_id::text,role FROM event_hosts WHERE event_id=$1 ORDER BY role,COALESCE(scene_id,profile_id)`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EventHost
	for rows.Next() {
		var v EventHost
		if err = rows.Scan(&v.EventID, &v.SceneID, &v.ProfileID, &v.Role); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *SQLRepository) UpsertSource(v Source) (Source, error) {
	if err := v.Validate(); err != nil {
		return Source{}, err
	}
	row := r.db.QueryRow(`INSERT INTO sources(id,provider,external_id,canonical_url,payload_sha256,first_seen_at,last_seen_at) VALUES($1,$2,$3,$4,$5,$6,$7)
	ON CONFLICT(provider,external_id,canonical_url) DO UPDATE SET last_seen_at=GREATEST(sources.last_seen_at,EXCLUDED.last_seen_at),payload_sha256=CASE WHEN EXCLUDED.last_seen_at>=sources.last_seen_at THEN EXCLUDED.payload_sha256 ELSE sources.payload_sha256 END
	RETURNING id::text,provider,external_id,canonical_url,payload_sha256,first_seen_at,last_seen_at`, v.ID, v.Provider, v.ExternalID, v.CanonicalURL, v.PayloadSHA256, v.FirstSeenAt, v.LastSeenAt)
	if err := row.Scan(&v.ID, &v.Provider, &v.ExternalID, &v.CanonicalURL, &v.PayloadSHA256, &v.FirstSeenAt, &v.LastSeenAt); err != nil {
		return Source{}, mapTouringError(err, ErrDuplicateSource)
	}
	return v, nil
}
func (r *SQLRepository) CreateAssertion(v EntityAssertion) error {
	if err := v.Validate(); err != nil {
		return err
	}
	fields, err := json.Marshal(v.AssertedFields)
	if err != nil {
		return ErrInvalidAssertion
	}
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if v.SupersedesID != nil {
		var entityType, entityID string
		if err = tx.QueryRow(`SELECT entity_type,entity_id::text FROM entity_assertions WHERE id=$1 FOR SHARE`, *v.SupersedesID).Scan(&entityType, &entityID); errors.Is(err, sql.ErrNoRows) {
			return ErrInvalidSupersession
		} else if err != nil {
			return err
		}
		if entityType != v.EntityType || entityID != v.EntityID {
			return ErrInvalidSupersession
		}
	}
	_, err = tx.Exec(`INSERT INTO entity_assertions(id,entity_type,entity_id,source_id,state,submitter_type,submitted_by_did,integration_id,authority_type,asserted_fields,rationale,reviewed_by_did,reviewed_at,asserted_at,supersedes_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`, v.ID, v.EntityType, v.EntityID, v.SourceID, v.State, v.SubmitterType, v.SubmittedByDID, v.IntegrationID, v.AuthorityType, fields, v.Rationale, v.ReviewedByDID, v.ReviewedAt, v.AssertedAt, v.SupersedesID)
	if err != nil {
		return mapTouringError(err, ErrDuplicateAssertion)
	}
	return tx.Commit()
}
func scanAssertion(row interface{ Scan(...any) error }) (EntityAssertion, error) {
	var v EntityAssertion
	var fields []byte
	err := row.Scan(&v.ID, &v.EntityType, &v.EntityID, &v.SourceID, &v.State, &v.SubmitterType, &v.SubmittedByDID, &v.IntegrationID, &v.AuthorityType, &fields, &v.Rationale, &v.ReviewedByDID, &v.ReviewedAt, &v.AssertedAt, &v.SupersedesID)
	if err == nil {
		err = json.Unmarshal(fields, &v.AssertedFields)
	}
	return v, err
}
func (r *SQLRepository) GetAssertion(id string) (EntityAssertion, error) {
	v, err := scanAssertion(r.db.QueryRow(`SELECT id::text,entity_type,entity_id::text,source_id::text,state,submitter_type,submitted_by_did,integration_id,authority_type,asserted_fields,COALESCE(rationale,''),reviewed_by_did,reviewed_at,asserted_at,supersedes_id::text FROM entity_assertions WHERE id=$1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return EntityAssertion{}, ErrAssertionNotFound
	}
	return v, err
}
func (r *SQLRepository) VerificationForEntity(kind, id string) string {
	var state string
	err := r.db.QueryRow(`SELECT state FROM entity_assertions WHERE entity_type=$1 AND entity_id=$2 ORDER BY asserted_at DESC,id DESC LIMIT 1`, kind, id).Scan(&state)
	if err != nil {
		return "unverified"
	}
	return state
}

func mapTouringError(err, errorKind error) error {
	if err == nil {
		return nil
	}
	var pqerr *pq.Error
	if errors.As(err, &pqerr) {
		switch pqerr.Code {
		case "23505":
			return errorKind
		case "23503":
			return errorKind
		case "23514":
			return errorKind
		}
	}
	return err
}

func defaultPublicationStatus(value string) string {
	if value == "published" || value == "archived" {
		return value
	}
	return "draft"
}

var _ Repository = (*SQLRepository)(nil)
