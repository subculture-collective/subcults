package main

import (
	"os"
	"strings"
	"testing"
)

func TestIndexerSourceDoesNotLogDatabaseURL(t *testing.T) {
	source, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}

	for _, forbidden := range []string{
		`"database_url"`,
		`LogSummary()["database_url"]`,
	} {
		if strings.Contains(string(source), forbidden) {
			t.Fatalf("indexer source must not log database URLs; found %q", forbidden)
		}
	}
}
