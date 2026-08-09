package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDurableBetaSurface(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	handler := DurableBetaSurface(next)
	for _, path := range []string{"/posts", "/scenes/id/memberships", "/streams/id", "/payments/checkout", "/api/telemetry"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusNotImplemented {
			t.Fatalf("%s status=%d", path, recorder.Code)
		}
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/search/appearances", nil))
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("durable route status=%d", recorder.Code)
	}
}
