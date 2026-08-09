// Package atprotocol defines Subcults' portable AT Protocol record contract.
package atprotocol

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
)

const (
	CanonicalPrefix = "tv.subcult."
	LegacyPrefix    = "app.subcult."

	CollectionProfile    = CanonicalPrefix + "profile"
	CollectionAct        = CanonicalPrefix + "act"
	CollectionPlace      = CanonicalPrefix + "place"
	CollectionVenue      = CanonicalPrefix + "venue"
	CollectionScene      = CanonicalPrefix + "scene"
	CollectionEvent      = CanonicalPrefix + "event"
	CollectionTour       = CanonicalPrefix + "tour"
	CollectionAppearance = CanonicalPrefix + "appearance"
	CollectionAssertion  = CanonicalPrefix + "assertion"
)

var (
	ErrUnsupportedCollection = errors.New("unsupported AT Protocol collection")
	ErrInvalidRecord         = errors.New("invalid AT Protocol record")
	ErrProtectedData         = errors.New("protected data cannot be published")
)

// CanonicalCollections is the exact least-privilege publication scope.
var CanonicalCollections = []string{
	CollectionProfile, CollectionAct, CollectionPlace, CollectionVenue,
	CollectionScene, CollectionEvent, CollectionTour, CollectionAppearance,
	CollectionAssertion,
}

var canonical = func() map[string]struct{} {
	result := make(map[string]struct{}, len(CanonicalCollections))
	for _, collection := range CanonicalCollections {
		result[collection] = struct{}{}
	}
	return result
}()

var forbiddenPublicKeys = map[string]struct{}{
	"precise_point": {}, "precisePoint": {}, "exact_address": {},
	"exactAddress": {}, "address_ciphertext": {}, "addressCiphertext": {},
	"encrypted_address": {}, "encryptedAddress": {}, "contact_email": {},
	"contactEmail": {}, "contact_phone": {}, "contactPhone": {},
}

// IsCanonicalCollection reports whether collection may be emitted by Subcults.
func IsCanonicalCollection(collection string) bool {
	_, ok := canonical[collection]
	return ok
}

// IsLegacyCollection reports whether collection is accepted only for migration.
func IsLegacyCollection(collection string) bool {
	switch collection {
	case "app.subcult.scene", "app.subcult.event", "app.subcult.post", "app.subcult.alliance":
		return true
	default:
		return false
	}
}

// PublishingScope returns the exact OAuth scope for Subcults public records.
func PublishingScope() string {
	parts := []string{"atproto"}
	for _, collection := range CanonicalCollections {
		parts = append(parts, "repo:"+collection)
	}
	return strings.Join(parts, " ")
}

// ValidatePublicRecord validates collection identity and rejects private fields
// recursively before a record can leave the application boundary.
func ValidatePublicRecord(collection string, payload []byte) error {
	if !IsCanonicalCollection(collection) {
		return fmt.Errorf("%w: %s", ErrUnsupportedCollection, collection)
	}
	var record map[string]any
	if err := json.Unmarshal(payload, &record); err != nil {
		return fmt.Errorf("%w: malformed JSON", ErrInvalidRecord)
	}
	if record["$type"] != collection {
		return fmt.Errorf("%w: $type must equal collection", ErrInvalidRecord)
	}
	if createdAt, ok := record["createdAt"].(string); !ok || strings.TrimSpace(createdAt) == "" {
		return fmt.Errorf("%w: createdAt is required", ErrInvalidRecord)
	}
	if err := rejectProtectedFields(record); err != nil {
		return err
	}
	if err := validateCanonicalShape(collection, record); err != nil {
		return err
	}
	return nil
}

func validateCanonicalShape(collection string, record map[string]any) error {
	required := map[string][]string{
		CollectionProfile: {"name", "kind"}, CollectionAct: {"name", "profile"},
		CollectionPlace: {"name", "country", "timezone", "coarseGeohash"},
		CollectionVenue: {"name", "place", "disclosure"}, CollectionScene: {"name", "coarseGeohash"},
		CollectionEvent:      {"title", "startsAt", "place", "disclosure"},
		CollectionTour:       {"title", "primaryAct", "status"},
		CollectionAppearance: {"event", "act", "role", "status"},
		CollectionAssertion:  {"target", "sourceUrl", "assertedAt", "observedAt", "status"},
	}
	for _, field := range append(required[collection], "createdAt") {
		value, ok := record[field]
		if !ok || value == nil || (isStringField(field) && strings.TrimSpace(fmt.Sprint(value)) == "") {
			return fmt.Errorf("%w: %s is required", ErrInvalidRecord, field)
		}
	}
	for _, field := range []string{"profile", "homeTerritory", "place", "venue", "primaryAct", "event", "act", "tour", "target", "supersedes"} {
		if value, ok := record[field]; ok {
			raw, stringOK := value.(string)
			if !stringOK {
				return fmt.Errorf("%w: %s must be an AT URI", ErrInvalidRecord, field)
			}
			uri, parseErr := syntax.ParseATURI(raw)
			if parseErr != nil || uri.RecordKey().String() == "" {
				return fmt.Errorf("%w: %s must be a record AT URI", ErrInvalidRecord, field)
			}
		}
	}
	for _, field := range []string{"createdAt", "updatedAt", "startsAt", "endsAt", "setStartsAt", "assertedAt", "observedAt", "effectiveAt"} {
		if value, ok := record[field]; ok {
			raw, stringOK := value.(string)
			if !stringOK {
				return fmt.Errorf("%w: %s must be a datetime", ErrInvalidRecord, field)
			}
			if _, parseErr := time.Parse(time.RFC3339Nano, raw); parseErr != nil {
				return fmt.Errorf("%w: %s must be RFC3339", ErrInvalidRecord, field)
			}
		}
	}
	return nil
}

func isStringField(field string) bool { return field != "confidence" }

func rejectProtectedFields(value any) error {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if _, forbidden := forbiddenPublicKeys[key]; forbidden {
				return fmt.Errorf("%w: %s", ErrProtectedData, key)
			}
			if err := rejectProtectedFields(child); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range typed {
			if err := rejectProtectedFields(child); err != nil {
				return err
			}
		}
	}
	return nil
}
