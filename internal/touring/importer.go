package touring

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidCSVHeader = errors.New("invalid touring CSV header")
	ErrInvalidCSVRow    = errors.New("invalid touring CSV row")
)

var strictCSVHeader = []string{
	"external_id", "title", "act_name", "venue_name", "place_name",
	"country_code", "timezone", "starts_at", "ends_at", "status", "source_url",
}

// CSVRow is the complete first-party interchange contract. Extra or missing
// columns are rejected before any assertion is written.
type CSVRow struct {
	ExternalID  string
	Title       string
	ActName     string
	VenueName   string
	PlaceName   string
	CountryCode string
	Timezone    string
	StartsAt    time.Time
	EndsAt      *time.Time
	Status      string
	SourceURL   string
}

type ImportResult struct {
	Accepted         int
	ReviewCandidates int
	AutoMerged       int
}

type assertionWriter interface {
	UpsertSource(Source) (Source, error)
	CreateAssertion(EntityAssertion) error
}

// Importer converts each row to a source assertion and never mutates an Event.
type Importer struct {
	repository assertionWriter
	now        func() time.Time
}

func NewImporter(repository assertionWriter) *Importer {
	return &Importer{repository: repository, now: func() time.Time { return time.Now().UTC() }}
}

func (i *Importer) ImportCSV(ctx context.Context, provider string, reader io.Reader) (ImportResult, error) {
	if strings.TrimSpace(provider) == "" {
		return ImportResult{}, fmt.Errorf("%w: provider", ErrInvalidCSVRow)
	}
	rows, err := decodeStrictCSV(reader)
	if err != nil {
		return ImportResult{}, err
	}
	result := ImportResult{}
	for _, row := range rows {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if err := i.normalizeAndAssert(provider, row); err != nil {
			return result, err
		}
		result.Accepted++
	}
	return result, nil
}

func decodeStrictCSV(reader io.Reader) ([]CSVRow, error) {
	decoder := csv.NewReader(reader)
	decoder.FieldsPerRecord = len(strictCSVHeader)
	header, err := decoder.Read()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidCSVHeader, err)
	}
	for index, expected := range strictCSVHeader {
		if strings.TrimSpace(header[index]) != expected {
			return nil, fmt.Errorf("%w: column %d must be %q", ErrInvalidCSVHeader, index+1, expected)
		}
	}

	rows := make([]CSVRow, 0)
	for line := 2; ; line++ {
		record, err := decoder.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%w: line %d: %v", ErrInvalidCSVRow, line, err)
		}
		row, err := parseCSVRow(record)
		if err != nil {
			return nil, fmt.Errorf("%w: line %d: %v", ErrInvalidCSVRow, line, err)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func parseCSVRow(record []string) (CSVRow, error) {
	row := CSVRow{
		ExternalID: strings.TrimSpace(record[0]), Title: strings.TrimSpace(record[1]),
		ActName: strings.TrimSpace(record[2]), VenueName: strings.TrimSpace(record[3]),
		PlaceName: strings.TrimSpace(record[4]), CountryCode: strings.ToUpper(strings.TrimSpace(record[5])),
		Timezone: strings.TrimSpace(record[6]), Status: strings.TrimSpace(record[9]),
		SourceURL: strings.TrimSpace(record[10]),
	}
	if row.ExternalID == "" || row.Title == "" || row.ActName == "" || row.PlaceName == "" ||
		len(row.CountryCode) != 2 || row.Timezone == "" || row.SourceURL == "" {
		return CSVRow{}, errors.New("required value missing")
	}
	if _, err := time.LoadLocation(row.Timezone); err != nil {
		return CSVRow{}, fmt.Errorf("timezone: %w", err)
	}
	parsedURL, err := url.ParseRequestURI(row.SourceURL)
	if err != nil || (parsedURL.Scheme != "https" && parsedURL.Scheme != "http") {
		return CSVRow{}, errors.New("source_url must be an HTTP(S) URL")
	}
	row.StartsAt, err = time.Parse(time.RFC3339, strings.TrimSpace(record[7]))
	if err != nil {
		return CSVRow{}, fmt.Errorf("starts_at: %w", err)
	}
	if value := strings.TrimSpace(record[8]); value != "" {
		endsAt, parseErr := time.Parse(time.RFC3339, value)
		if parseErr != nil || !endsAt.After(row.StartsAt) {
			return CSVRow{}, errors.New("ends_at must be RFC3339 and after starts_at")
		}
		row.EndsAt = &endsAt
	}
	switch row.Status {
	case "announced", "confirmed", "cancelled", "completed":
	default:
		return CSVRow{}, errors.New("unsupported status")
	}
	return row, nil
}

func (i *Importer) normalizeAndAssert(provider string, row CSVRow) error {
	now := i.now()
	payload := strings.Join([]string{
		row.ExternalID, row.Title, row.ActName, row.VenueName, row.PlaceName,
		row.CountryCode, row.Timezone, row.StartsAt.Format(time.RFC3339), row.Status, row.SourceURL,
	}, "\x00")
	digest := sha256.Sum256([]byte(payload))
	externalID := row.ExternalID
	canonicalURL := row.SourceURL
	sourceID := uuid.NewSHA1(uuid.NameSpaceURL, []byte(strings.ToLower(provider)+"\x00"+row.ExternalID)).String()
	source, err := i.repository.UpsertSource(Source{
		ID: sourceID, Provider: provider, ExternalID: &externalID, CanonicalURL: &canonicalURL,
		PayloadSHA256: hex.EncodeToString(digest[:]), FirstSeenAt: now, LastSeenAt: now,
	})
	if err != nil {
		return err
	}
	entityID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(strings.ToLower(provider)+"\x00"+row.ExternalID)).String()
	integrationID := provider
	return i.repository.CreateAssertion(EntityAssertion{
		ID:         uuid.NewSHA1(uuid.NameSpaceX500, []byte(source.ID+"\x00"+hex.EncodeToString(digest[:]))).String(),
		EntityType: "event", EntityID: entityID, SourceID: source.ID, State: "unverified",
		SubmitterType: "integration", IntegrationID: &integrationID, AuthorityType: "ticketing_provider",
		AssertedFields: map[string]any{
			"external_id": row.ExternalID, "title": row.Title, "act_name": row.ActName,
			"venue_name": row.VenueName, "place_name": row.PlaceName, "country_code": row.CountryCode,
			"timezone": row.Timezone, "starts_at": row.StartsAt.Format(time.RFC3339),
			"ends_at": row.EndsAt, "status": row.Status, "source_url": row.SourceURL,
		},
		AssertedAt: now,
	})
}
