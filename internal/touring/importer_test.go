package touring

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

const csvHeader = "external_id,title,act_name,venue_name,place_name,country_code,timezone,starts_at,ends_at,status,source_url\n"

func TestImporterCreatesSourceAssertionWithoutCanonicalEventMutation(t *testing.T) {
	repository := NewInMemoryRepository()
	importer := NewImporter(repository)
	importer.now = func() time.Time { return time.Date(2026, time.August, 8, 12, 0, 0, 0, time.UTC) }
	data := csvHeader + "event-1,Away Show,Example Act,Metro,Chicago,US,America/Chicago,2026-09-01T20:00:00-05:00,,confirmed,https://tickets.example/events/1\n"
	result, err := importer.ImportCSV(context.Background(), "ticketing", strings.NewReader(data))
	if err != nil {
		t.Fatalf("ImportCSV: %v", err)
	}
	if result.Accepted != 1 || len(repository.sources) != 1 || len(repository.assertions) != 1 || len(repository.appearances) != 0 {
		t.Fatalf("result=%+v sources=%d assertions=%d appearances=%d", result, len(repository.sources), len(repository.assertions), len(repository.appearances))
	}
}

func TestImporterRejectsAdditionalColumnsBeforeWriting(t *testing.T) {
	repository := NewInMemoryRepository()
	importer := NewImporter(repository)
	data := strings.TrimSuffix(csvHeader, "\n") + ",unexpected\n"
	_, err := importer.ImportCSV(context.Background(), "ticketing", strings.NewReader(data))
	if !errors.Is(err, ErrInvalidCSVHeader) {
		t.Fatalf("error=%v, want ErrInvalidCSVHeader", err)
	}
	if len(repository.sources) != 0 {
		t.Fatal("invalid header wrote source state")
	}
}

func TestImporterDoesNotMergeAmbiguousFestivalAfterparty(t *testing.T) {
	result := Reconcile([]ReconciliationCandidate{
		{Provider: "festival", ExternalID: "main", ActName: "Example Act", LocalDate: "2026-09-01", PlaceName: "Chicago", VenueName: "Metro", EventKind: "festival"},
		{Provider: "afterparty", ExternalID: "late", ActName: "Example Act", LocalDate: "2026-09-01", PlaceName: "Chicago", VenueName: "Metro", EventKind: "party"},
	})
	if result.AutoMerged != 0 || result.ReviewCandidates != 1 {
		t.Fatalf("result=%+v", result)
	}
}
