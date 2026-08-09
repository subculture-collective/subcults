package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateDoesNotReplaceExistingSecretWithoutForce(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("ATPROTO_SESSION_ENCRYPTION_KEY=keep-me\nPDS_PROVISIONER_TOKEN=\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := update(path, false); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "ATPROTO_SESSION_ENCRYPTION_KEY=keep-me") {
		t.Fatal("existing secret was replaced")
	}
	if strings.Contains(text, "PDS_PROVISIONER_TOKEN=\n") {
		t.Fatal("blank provisioner token was not populated")
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, err=%v", info.Mode().Perm(), err)
	}
}
