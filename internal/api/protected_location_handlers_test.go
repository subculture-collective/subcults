package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/onnwee/subcults/internal/locationaccess"
	"github.com/onnwee/subcults/internal/middleware"
	"github.com/onnwee/subcults/internal/scene"
)

func TestProtectedLocationRequiresExplicitGrant(t *testing.T) {
	scenes := scene.NewInMemorySceneRepository()
	events := scene.NewInMemoryEventRepository()
	grants := locationaccess.NewInMemoryRepository()
	point := &scene.Point{Lat: 41.88, Lng: -87.63}
	if err := scenes.Insert(&scene.Scene{ID: "scene-1", Name: "Night Signal", OwnerDID: "did:web:owner", CoarseGeohash: "dp3wj", Visibility: scene.VisibilityPublic}); err != nil {
		t.Fatal(err)
	}
	if err := events.Insert(&scene.Event{ID: "event-1", SceneID: "scene-1", Title: "Protected show", AllowPrecise: true, PrecisePoint: point, CoarseGeohash: "dp3wj", LocationAccess: "protected", StartsAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	handler := NewProtectedLocationHandlers(grants, events, scenes)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/events/event-1/location", nil)
	ctx := middleware.SetUserID(request.Context(), "user-1")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request.WithContext(ctx))
	if response.Code != http.StatusForbidden {
		t.Fatalf("without grant: got %d", response.Code)
	}

	if err := grants.Grant(context.Background(), locationaccess.Grant{EventID: "event-1", UserID: "user-1", Reason: "rsvp", GrantedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request.WithContext(ctx))
	if response.Code != http.StatusOK {
		t.Fatalf("with grant: got %d: %s", response.Code, response.Body.String())
	}
	var envelope struct {
		Data struct {
			Point scene.Point `json:"point"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.Point != *point {
		t.Fatalf("got point %#v", envelope.Data.Point)
	}
}

func TestPublicEventCopyHidesProtectedPrecisePoint(t *testing.T) {
	event := &scene.Event{LocationAccess: "protected", AllowPrecise: true, PrecisePoint: &scene.Point{Lat: 1, Lng: 2}}
	public := publicEventCopy(event)
	if public.PrecisePoint != nil {
		t.Fatal("protected precise point leaked")
	}
	if event.PrecisePoint == nil {
		t.Fatal("sanitizer mutated stored event")
	}
}
