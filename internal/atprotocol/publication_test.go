package atprotocol

import (
	"encoding/json"
	"testing"
)

func TestEntityCollectionMatches(t *testing.T) {
	if !entityCollectionMatches("event", CollectionEvent) {
		t.Fatal("event collection should match")
	}
	if entityCollectionMatches("event", CollectionScene) {
		t.Fatal("event must not publish as a scene")
	}
}

func TestPublicEventCannotContainProtectedCoordinates(t *testing.T) {
	record := map[string]any{
		"$type": CollectionEvent, "createdAt": "2026-08-09T00:00:00Z",
		"title": "Secret floor", "precise_point": map[string]any{"lat": 1, "lng": 2},
	}
	payload, _ := json.Marshal(record)
	if err := ValidatePublicRecord(CollectionEvent, payload); err == nil {
		t.Fatal("protected coordinates were accepted")
	}
}
