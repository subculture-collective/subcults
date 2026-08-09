// Command atproto-keygen creates independent AT Protocol credentials and writes
// them into an ignored env file without printing secret values.
package main

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/bluesky-social/indigo/atproto/atcrypto"
)

func main() {
	path := flag.String("env", "deploy/.env", "env file to update")
	force := flag.Bool("force", false, "replace existing non-empty values")
	flag.Parse()
	if err := update(*path, *force); err != nil {
		fmt.Fprintln(os.Stderr, "AT Protocol credential generation failed:", err)
		os.Exit(1)
	}
	fmt.Println("AT Protocol credentials are present in", *path, "(values not displayed)")
}

func update(path string, force bool) error {
	privateKey, err := atcrypto.GeneratePrivateKeyP256()
	if err != nil {
		return err
	}
	session, err := randomBase64(32)
	if err != nil {
		return err
	}
	provisioner, err := randomBase64(32)
	if err != nil {
		return err
	}
	tapPassword, err := randomBase64(32)
	if err != nil {
		return err
	}
	values := map[string]string{
		"ATPROTO_OAUTH_CLIENT_PRIVATE_KEY": privateKey.Multibase(),
		"ATPROTO_OAUTH_CLIENT_KEY_ID":      "subcults-1",
		"ATPROTO_SESSION_ENCRYPTION_KEY":   session,
		"PDS_PROVISIONER_TOKEN":            provisioner,
		"ATPROTO_TAP_ADMIN_PASSWORD":       tapPassword,
	}

	original, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	lines := []string{}
	found := map[string]bool{}
	scanner := bufio.NewScanner(strings.NewReader(string(original)))
	for scanner.Scan() {
		line := scanner.Text()
		key, current, ok := strings.Cut(line, "=")
		if replacement, managed := values[key]; managed && ok {
			found[key] = true
			if force || strings.TrimSpace(current) == "" {
				line = key + "=" + replacement
			}
		}
		lines = append(lines, line)
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	for _, key := range []string{"ATPROTO_OAUTH_CLIENT_PRIVATE_KEY", "ATPROTO_OAUTH_CLIENT_KEY_ID", "ATPROTO_SESSION_ENCRYPTION_KEY", "PDS_PROVISIONER_TOKEN", "ATPROTO_TAP_ADMIN_PASSWORD"} {
		if !found[key] {
			lines = append(lines, key+"="+values[key])
		}
	}
	output := strings.Join(lines, "\n") + "\n"
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, []byte(output), 0o600); err != nil {
		return err
	}
	if err := os.Chmod(temporary, 0o600); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return os.Rename(temporary, path)
}

func randomBase64(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(value), nil
}
