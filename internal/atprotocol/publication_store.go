package atprotocol

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"
)

// PublicRecordForEntity is the only serializer used by Studio publishing. It
// deliberately selects disclosure-safe columns rather than serializing domain
// structs that may also contain private coordinates or authorization data.
func (s *SQLStore) PublicRecordForEntity(ctx context.Context, userID, entityType, entityID string) (string, []byte, error) {
	owned, err := s.userOwnsEntity(ctx, userID, entityType, entityID)
	if err != nil {
		return "", nil, err
	}
	if !owned {
		return "", nil, ErrEntityForbidden
	}

	record := map[string]any{}
	var collection string
	switch entityType {
	case "scene":
		collection = CollectionScene
		var name, description, geohash, visibility string
		var tags pq.StringArray
		var created, updated time.Time
		err = s.db.QueryRowContext(ctx, `SELECT name, COALESCE(description,''), coarse_geohash,
			COALESCE(tags, ARRAY[]::text[]), visibility, created_at, updated_at
			FROM scenes WHERE id=$1::uuid AND deleted_at IS NULL`, entityID).
			Scan(&name, &description, &geohash, &tags, &visibility, &created, &updated)
		record = map[string]any{"$type": collection, "name": name, "description": description,
			"coarseGeohash": geohash, "tags": []string(tags), "visibility": visibility,
			"createdAt": created.UTC().Format(time.RFC3339Nano), "updatedAt": updated.UTC().Format(time.RFC3339Nano)}
	case "profile":
		collection = CollectionProfile
		var name, kind string
		var created, updated time.Time
		err = s.db.QueryRowContext(ctx, `SELECT canonical_name, kind, created_at, updated_at FROM profiles WHERE id=$1::uuid`, entityID).
			Scan(&name, &kind, &created, &updated)
		record = map[string]any{"$type": collection, "name": name, "kind": kind,
			"createdAt": created.UTC().Format(time.RFC3339Nano), "updatedAt": updated.UTC().Format(time.RFC3339Nano)}
	case "place":
		collection = CollectionPlace
		var name, region, country, timezone, geohash string
		var created, updated time.Time
		err = s.db.QueryRowContext(ctx, `SELECT canonical_name, COALESCE(admin_region,''), country_code,
			timezone, coarse_geohash, created_at, updated_at FROM places WHERE id=$1::uuid`, entityID).
			Scan(&name, &region, &country, &timezone, &geohash, &created, &updated)
		record = map[string]any{"$type": collection, "name": name, "region": region, "country": country,
			"timezone": timezone, "coarseGeohash": geohash,
			"createdAt": created.UTC().Format(time.RFC3339Nano), "updatedAt": updated.UTC().Format(time.RFC3339Nano)}
	case "venue":
		collection = CollectionVenue
		var name, placeID string
		var created, updated time.Time
		err = s.db.QueryRowContext(ctx, `SELECT canonical_name, place_id::text, created_at, updated_at
			FROM venues WHERE id=$1::uuid`, entityID).Scan(&name, &placeID, &created, &updated)
		if err == nil {
			var placeURI string
			placeURI, err = s.entityATURI(ctx, "place", placeID)
			record = map[string]any{"$type": collection, "name": name, "place": placeURI,
				"disclosure": "coarse",
				"createdAt":  created.UTC().Format(time.RFC3339Nano), "updatedAt": updated.UTC().Format(time.RFC3339Nano)}
		}
	case "act":
		collection = CollectionAct
		var name, profileID string
		var created time.Time
		err = s.db.QueryRowContext(ctx, `SELECT p.canonical_name, a.profile_id::text, a.created_at
			FROM acts a JOIN profiles p ON p.id=a.profile_id WHERE a.id=$1::uuid`, entityID).
			Scan(&name, &profileID, &created)
		if err == nil {
			var profileURI string
			profileURI, err = s.entityATURI(ctx, "profile", profileID)
			record = map[string]any{"$type": collection, "name": name, "profile": profileURI,
				"createdAt": created.UTC().Format(time.RFC3339Nano), "updatedAt": created.UTC().Format(time.RFC3339Nano)}
		}
	case "event":
		collection = CollectionEvent
		var title, description, geohash, kind, status, locationAccess, placeID string
		var venueID sql.NullString
		var starts time.Time
		var ends, created, updated sql.NullTime
		err = s.db.QueryRowContext(ctx, `SELECT title, COALESCE(description,''), starts_at, ends_at,
			coarse_geohash, kind, status, location_access, place_id::text, venue_id::text,
			created_at, updated_at FROM events WHERE id=$1::uuid AND deleted_at IS NULL`, entityID).
			Scan(&title, &description, &starts, &ends, &geohash, &kind, &status, &locationAccess,
				&placeID, &venueID, &created, &updated)
		if err == nil {
			var placeURI string
			placeURI, err = s.entityATURI(ctx, "place", placeID)
			record = map[string]any{"$type": collection, "title": title, "description": description,
				"startsAt": starts.UTC().Format(time.RFC3339Nano), "place": placeURI, "kind": kind,
				"status": publicEventStatus(status), "disclosure": publicDisclosure(locationAccess),
				"coarseGeohash": geohash, "createdAt": nullTimeString(created, starts),
				"updatedAt": nullTimeString(updated, starts)}
			if ends.Valid {
				record["endsAt"] = ends.Time.UTC().Format(time.RFC3339Nano)
			}
			if venueID.Valid {
				if venueURI, venueErr := s.entityATURI(ctx, "venue", venueID.String); venueErr == nil {
					record["venue"] = venueURI
				}
			}
		}
	case "tour":
		collection = CollectionTour
		var title, status, actID string
		var created, updated time.Time
		err = s.db.QueryRowContext(ctx, `SELECT title, status, primary_act_id::text, created_at, updated_at FROM tours WHERE id=$1::uuid`, entityID).
			Scan(&title, &status, &actID, &created, &updated)
		if err == nil {
			var actURI string
			actURI, err = s.entityATURI(ctx, "act", actID)
			record = map[string]any{"$type": collection, "title": title, "status": status, "primaryAct": actURI,
				"createdAt": created.UTC().Format(time.RFC3339Nano), "updatedAt": updated.UTC().Format(time.RFC3339Nano)}
		}
	case "appearance":
		collection = CollectionAppearance
		var eventID, actID, role, status, eventKind string
		var tourID sql.NullString
		var starts sql.NullTime
		var created, updated time.Time
		err = s.db.QueryRowContext(ctx, `SELECT a.event_id::text, a.act_id::text, a.tour_id::text,
			a.role, a.status, a.starts_at, a.created_at, a.updated_at, e.kind
			FROM appearances a JOIN events e ON e.id=a.event_id WHERE a.id=$1::uuid`, entityID).
			Scan(&eventID, &actID, &tourID, &role, &status, &starts, &created, &updated, &eventKind)
		if err == nil {
			var eventURI, actURI string
			eventURI, err = s.entityATURI(ctx, "event", eventID)
			if err == nil {
				actURI, err = s.entityATURI(ctx, "act", actID)
			}
			record = map[string]any{"$type": collection, "event": eventURI, "act": actURI, "role": role,
				"billingContext": billingContext(tourID.Valid, eventKind), "status": publicAppearanceStatus(status),
				"createdAt": created.UTC().Format(time.RFC3339Nano), "updatedAt": updated.UTC().Format(time.RFC3339Nano)}
			if starts.Valid {
				record["setStartsAt"] = starts.Time.UTC().Format(time.RFC3339Nano)
			}
			if tourID.Valid && err == nil {
				record["tour"], err = s.entityATURI(ctx, "tour", tourID.String)
			}
		}
	default:
		return "", nil, ErrUnsupportedCollection
	}
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil, ErrOAuthRequestNotFound
		}
		return "", nil, err
	}
	payload, err := json.Marshal(record)
	return collection, payload, err
}

func (s *SQLStore) entityATURI(ctx context.Context, entityType, entityID string) (string, error) {
	var uri string
	err := s.db.QueryRowContext(ctx, `SELECT at_uri FROM atproto_record_mappings
		WHERE entity_type=$1 AND entity_id=$2::uuid AND projection_status NOT IN ('deleted','failed')`, entityType, entityID).Scan(&uri)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrRecordConflict
	}
	return uri, err
}

func nullTimeString(value sql.NullTime, fallback time.Time) string {
	if value.Valid {
		return value.Time.UTC().Format(time.RFC3339Nano)
	}
	return fallback.UTC().Format(time.RFC3339Nano)
}

func publicEventStatus(status string) string {
	switch status {
	case "cancelled":
		return "cancelled"
	case "ended":
		return "completed"
	default:
		return "announced"
	}
}
func publicDisclosure(access string) string {
	if access == "protected" {
		return "protected"
	}
	return "coarse"
}
func publicAppearanceStatus(status string) string {
	if status == "confirmed" {
		return "announced"
	}
	return status
}
func billingContext(hasTour bool, eventKind string) string {
	if hasTour {
		return "tour_stop"
	}
	if eventKind == "festival" {
		return "festival"
	}
	return "one_off"
}

// ReserveRecord authorizes the local entity and creates its stable PDS mapping.
func (s *SQLStore) ReserveRecord(ctx context.Context, userID, entityType, entityID, collection, publisherDID, rkey string) (RecordMapping, error) {
	owned, err := s.userOwnsEntity(ctx, userID, entityType, entityID)
	if err != nil {
		return RecordMapping{}, err
	}
	if !owned {
		return RecordMapping{}, ErrEntityForbidden
	}
	atURI := fmt.Sprintf("at://%s/%s/%s", publisherDID, collection, rkey)
	var mapping RecordMapping
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO atproto_record_mappings (
			entity_type, entity_id, publisher_did, collection, rkey, at_uri,
			projection_status, updated_at
		) VALUES ($1, $2::uuid, $3, $4, $5, $6, 'reserved', $7)
		ON CONFLICT (entity_type, entity_id) DO UPDATE
		SET updated_at = atproto_record_mappings.updated_at
		WHERE atproto_record_mappings.publisher_did = EXCLUDED.publisher_did
		  AND atproto_record_mappings.collection = EXCLUDED.collection
		RETURNING entity_type, entity_id::text, publisher_did, collection, rkey,
		          at_uri, COALESCE(cid, ''), projection_status, record_version, updated_at
	`, entityType, entityID, publisherDID, collection, rkey, atURI, s.now().UTC()).Scan(
		&mapping.EntityType, &mapping.EntityID, &mapping.PublisherDID,
		&mapping.Collection, &mapping.RKey, &mapping.ATURI, &mapping.CID,
		&mapping.ProjectionStatus, &mapping.RecordVersion, &mapping.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return RecordMapping{}, ErrRecordConflict
	}
	if err != nil {
		return RecordMapping{}, fmt.Errorf("reserve AT Protocol record: %w", err)
	}
	return mapping, nil
}

func (s *SQLStore) userOwnsEntity(ctx context.Context, userID, entityType, entityID string) (bool, error) {
	var query string
	switch entityType {
	case "scene":
		query = `SELECT EXISTS (
			SELECT 1 FROM scenes s JOIN users u ON u.internal_did = s.owner_did
			WHERE s.id = $2::uuid AND u.id = $1::uuid AND s.deleted_at IS NULL
		)`
	case "event":
		query = `SELECT EXISTS (
			SELECT 1 FROM events e JOIN scenes s ON s.id = e.scene_id
			JOIN users u ON u.internal_did = s.owner_did
			WHERE e.id = $2::uuid AND u.id = $1::uuid AND e.deleted_at IS NULL
		)`
	case "profile":
		query = "SELECT EXISTS (SELECT 1 FROM profiles WHERE id=$2::uuid AND created_by_user_id=$1::uuid)"
	case "place":
		query = "SELECT EXISTS (SELECT 1 FROM places WHERE id=$2::uuid AND created_by_user_id=$1::uuid)"
	case "venue":
		query = `SELECT EXISTS (
			SELECT 1 FROM venues v JOIN places p ON p.id=v.place_id
			WHERE v.id=$2::uuid AND p.created_by_user_id=$1::uuid
		)`
	case "act":
		query = `SELECT EXISTS (
			SELECT 1 FROM acts a JOIN profiles p ON p.id = a.profile_id
			WHERE a.id=$2::uuid AND p.created_by_user_id=$1::uuid
		)`
	case "tour":
		query = "SELECT EXISTS (SELECT 1 FROM tours WHERE id=$2::uuid AND created_by_user_id=$1::uuid)"
	case "appearance":
		query = "SELECT EXISTS (SELECT 1 FROM appearances WHERE id=$2::uuid AND created_by_user_id=$1::uuid)"
	default:
		return false, ErrEntityForbidden
	}
	var owned bool
	if err := s.db.QueryRowContext(ctx, query, userID, entityID).Scan(&owned); err != nil {
		return false, fmt.Errorf("authorize AT Protocol entity: %w", err)
	}
	return owned, nil
}

// MarkRecordAwaiting advances the mapping only after a successful PDS write.
func (s *SQLStore) MarkRecordAwaiting(ctx context.Context, atURI, cid string, expectedVersion int64) (RecordMapping, error) {
	var mapping RecordMapping
	err := s.db.QueryRowContext(ctx, `
		UPDATE atproto_record_mappings
		SET cid=$2, projection_status='awaiting_projection',
		    record_version=record_version+1, updated_at=$4
		WHERE at_uri=$1 AND record_version=$3
		RETURNING entity_type, entity_id::text, publisher_did, collection, rkey,
		          at_uri, cid, projection_status, record_version, updated_at
	`, atURI, cid, expectedVersion, s.now().UTC()).Scan(
		&mapping.EntityType, &mapping.EntityID, &mapping.PublisherDID,
		&mapping.Collection, &mapping.RKey, &mapping.ATURI, &mapping.CID,
		&mapping.ProjectionStatus, &mapping.RecordVersion, &mapping.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return RecordMapping{}, ErrRecordConflict
	}
	if err != nil {
		return RecordMapping{}, fmt.Errorf("mark AT Protocol projection pending: %w", err)
	}
	return mapping, nil
}

// MarkRecordFailed records publication failure without storing secret details.
func (s *SQLStore) MarkRecordFailed(ctx context.Context, atURI string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE atproto_record_mappings SET projection_status='failed', updated_at=$2
		WHERE at_uri=$1 AND projection_status IN ('reserved','awaiting_projection')
	`, atURI, s.now().UTC())
	return err
}

// RecordByURI returns projection state.
func (s *SQLStore) RecordByURI(ctx context.Context, atURI string) (RecordMapping, error) {
	var mapping RecordMapping
	err := s.db.QueryRowContext(ctx, `
		SELECT entity_type, entity_id::text, publisher_did, collection, rkey,
		       at_uri, COALESCE(cid,''), projection_status, record_version, updated_at
		FROM atproto_record_mappings WHERE at_uri=$1
	`, atURI).Scan(
		&mapping.EntityType, &mapping.EntityID, &mapping.PublisherDID,
		&mapping.Collection, &mapping.RKey, &mapping.ATURI, &mapping.CID,
		&mapping.ProjectionStatus, &mapping.RecordVersion, &mapping.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return RecordMapping{}, ErrOAuthRequestNotFound
	}
	return mapping, err
}

// MarkProjected is used by Tap, Jetstream shadow intake, or reconciliation.
func (s *SQLStore) MarkProjected(ctx context.Context, atURI, cid string, observedAt time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var entityType, entityID string
	if err = tx.QueryRowContext(ctx, `SELECT entity_type, entity_id::text FROM atproto_record_mappings WHERE at_uri=$1 FOR UPDATE`, atURI).Scan(&entityType, &entityID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrOAuthRequestNotFound
		}
		return err
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE atproto_record_mappings
		SET cid=$2, projection_status='projected', last_seen_at=$3, updated_at=$3
		WHERE at_uri=$1
	`, atURI, cid, observedAt.UTC())
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows != 1 {
		return ErrOAuthRequestNotFound
	}
	tableByType := map[string]string{
		"scene": "scenes", "event": "events", "profile": "profiles", "place": "places",
		"venue": "venues", "act": "acts", "tour": "tours", "appearance": "appearances",
	}
	if table := tableByType[entityType]; table != "" {
		if _, err = tx.ExecContext(ctx, "UPDATE "+table+" SET publication_status='published' WHERE id=$1::uuid", entityID); err != nil {
			return err
		}
	}
	return tx.Commit()
}
