// Package atprotocol defines Subcults' portable AT Protocol record contract.
package atprotocol

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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
	return nil
}

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
