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
	_, err := r.db.Exec(`INSERT INTO places(id,canonical_name,admin_region,country_code,timezone,coarse_geohash,version)
		VALUES($1,$2,NULLIF($3,''),upper($4),$5,$6,$7)`, v.ID, v.CanonicalName, v.AdminRegion, v.CountryCode, v.Timezone, v.CoarseGeohash, v.Version)
	return mapTouringError(err, ErrInvalidPlace)
}

func (r *SQLRepository) StoreProfile(v Profile) error {
	if err := v.Validate(); err != nil || strings.TrimSpace(v.ID) == "" {
		return ErrInvalidProfile
	}
	if v.Version == 0 {
		v.Version = 1
	}
	_, err := r.db.Exec(`INSERT INTO profiles(id,kind,canonical_name,visibility,version) VALUES($1,$2,$3,$4,$5)`, v.ID, v.Kind, v.CanonicalName, v.Visibility, v.Version)
	return mapTouringError(err, ErrInvalidProfile)
}
func (r *SQLRepository) StoreAct(v Act) error {
	if strings.TrimSpace(v.ID) == "" || strings.TrimSpace(v.ProfileID) == "" {
		return ErrInvalidProfile
	}
	_, err := r.db.Exec(`INSERT INTO acts(id,profile_id) VALUES($1,$2)`, v.ID, v.ProfileID)
	return mapTouringError(err, ErrInvalidProfile)
}
func (r *SQLRepository) UpdatePlace(v *Place) error {
	if err := v.Validate(); err != nil {
		return err
	}
	return scanVersion(r.db.QueryRow(`UPDATE places SET canonical_name=$2,admin_region=NULLIF($3,''),country_code=upper($4),timezone=$5,coarse_geohash=$6,updated_at=NOW(),version=version+1 WHERE id=$1 AND version=$7 RETURNING version`, v.ID, v.CanonicalName, v.AdminRegion, v.CountryCode, v.Timezone, v.CoarseGeohash, v.Version), &v.Version)
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
	err := r.db.QueryRow(`SELECT id::text,canonical_name,COALESCE(admin_region,''),country_code,timezone,coarse_geohash,version FROM places WHERE id=$1`, id).Scan(&v.ID, &v.CanonicalName, &v.AdminRegion, &v.CountryCode, &v.Timezone, &v.CoarseGeohash, &v.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return Place{}, ErrInvalidPlace
	}
	return v, err
}
func (r *SQLRepository) GetProfile(id string) (Profile, error) {
	var v Profile
	err := r.db.QueryRow(`SELECT id::text,kind,canonical_name,visibility,version FROM profiles WHERE id=$1`, id).Scan(&v.ID, &v.Kind, &v.CanonicalName, &v.Visibility, &v.Version)
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
	_, err = tx.Exec(`INSERT INTO tours(id,primary_act_id,title,status,starts_on,ends_on,version) VALUES($1,$2,$3,$4,$5,$6,$7)`, v.ID, v.PrimaryActID, v.Title, v.Status, v.StartsOn, v.EndsOn, v.Version)
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
	err := r.db.QueryRow(`SELECT id::text,primary_act_id::text,title,status,starts_on,ends_on,version FROM tours WHERE id=$1`, id).Scan(&v.ID, &v.PrimaryActID, &v.Title, &v.Status, &v.StartsOn, &v.EndsOn, &v.Version)
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
	_, err := r.db.Exec(`INSERT INTO appearances(id,event_id,act_id,tour_id,role,stage_name,starts_at,ends_at,status,version) VALUES($1,$2,$3,$4,$5,NULLIF($6,''),$7,$8,$9,$10)`, v.ID, v.EventID, v.ActID, v.TourID, v.Role, v.StageName, v.StartsAt, v.EndsAt, v.Status, v.Version)
	return mapTouringError(err, ErrDuplicateAppearance)
}
func scanAppearance(row interface{ Scan(...any) error }) (Appearance, error) {
	var v Appearance
	err := row.Scan(&v.ID, &v.EventID, &v.ActID, &v.TourID, &v.Role, &v.StageName, &v.StartsAt, &v.EndsAt, &v.Status, &v.Version)
	return v, err
}

const appearanceSelect = `SELECT id::text,event_id::text,act_id::text,tour_id::text,role,COALESCE(stage_name,''),starts_at,ends_at,status,version FROM appearances`

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

var _ Repository = (*SQLRepository)(nil)
