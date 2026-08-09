package atprotocol_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/onnwee/subcults/internal/atprotocol"
)

func TestCanonicalPublicationContract(t *testing.T) {
	t.Run("uses exact canonical scopes", func(t *testing.T) {
		scope := atprotocol.PublishingScope()
		if strings.Contains(scope, "transition:generic") || strings.Contains(scope, "app.subcult") {
			t.Fatalf("scope grants legacy or generic access: %q", scope)
		}
		for _, collection := range atprotocol.CanonicalCollections {
			if !strings.Contains(scope, "repo:"+collection) {
				t.Fatalf("scope omits %s", collection)
			}
		}
	})

	t.Run("rejects protected fields recursively", func(t *testing.T) {
		payload := []byte("{\"$type\":\"tv.subcult.event\",\"createdAt\":\"2026-08-09T00:00:00Z\",\"occurrence\":{\"precisePoint\":{\"lat\":1,\"lng\":2}}}")
		err := atprotocol.ValidatePublicRecord(atprotocol.CollectionEvent, payload)
		if !errors.Is(err, atprotocol.ErrProtectedData) {
			t.Fatalf("expected protected-data rejection, got %v", err)
		}
	})

	t.Run("accepts disclosure safe event", func(t *testing.T) {
		payload := []byte("{\"$type\":\"tv.subcult.event\",\"createdAt\":\"2026-08-09T00:00:00Z\",\"title\":\"Night Signal\",\"startsAt\":\"2026-08-10T00:00:00Z\",\"place\":\"at://did:plc:abc/tv.subcult.place/3kxyz\",\"disclosure\":\"coarse\",\"coarseGeohash\":\"dp3wj\"}")
		if err := atprotocol.ValidatePublicRecord(atprotocol.CollectionEvent, payload); err != nil {
			t.Fatalf("valid record rejected: %v", err)
		}
	})

	t.Run("rejects local UUID relationship", func(t *testing.T) {
		payload := []byte("{\"$type\":\"tv.subcult.event\",\"createdAt\":\"2026-08-09T00:00:00Z\",\"title\":\"Night Signal\",\"startsAt\":\"2026-08-10T00:00:00Z\",\"place\":\"48b3fc42-b532-40a5-a6ee-373fe6e36f7b\",\"disclosure\":\"coarse\"}")
		if err := atprotocol.ValidatePublicRecord(atprotocol.CollectionEvent, payload); !errors.Is(err, atprotocol.ErrInvalidRecord) {
			t.Fatalf("expected portable reference rejection, got %v", err)
		}
	})

	t.Run("legacy is intake only", func(t *testing.T) {
		if !atprotocol.IsLegacyCollection("app.subcult.scene") {
			t.Fatal("legacy scene should remain readable")
		}
		if atprotocol.IsCanonicalCollection("app.subcult.scene") {
			t.Fatal("legacy scene must never be publishable")
		}
	})
}
