// Command generate-local-secrets creates a local deployment environment file
// with cryptographically random application and Web Push credentials.
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	templatePath = "deploy/.env.example"
	outputPath   = "deploy/.env"
)

func randomBytes(size int) []byte {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		panic(fmt.Errorf("generate random bytes: %w", err))
	}
	return value
}

func main() {
	if _, err := os.Stat(outputPath); err == nil {
		fmt.Fprintf(os.Stderr, "%s already exists; refusing to overwrite it\n", outputPath)
		os.Exit(1)
	} else if !os.IsNotExist(err) {
		panic(err)
	}

	template, err := os.ReadFile(templatePath)
	if err != nil {
		panic(fmt.Errorf("read template: %w", err))
	}

	vapid, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(fmt.Errorf("generate VAPID key: %w", err))
	}
	privateBytes := vapid.D.FillBytes(make([]byte, 32))
	publicBytes := elliptic.Marshal(elliptic.P256(), vapid.PublicKey.X, vapid.PublicKey.Y)

	values := map[string]string{
		"JWT_SECRET_CURRENT":     base64.RawURLEncoding.EncodeToString(randomBytes(48)),
		"CONTACT_ENCRYPTION_KEY": base64.StdEncoding.EncodeToString(randomBytes(32)),
		"CONTACT_HMAC_KEY":       base64.StdEncoding.EncodeToString(randomBytes(32)),
		"VAPID_PUBLIC_KEY":       base64.RawURLEncoding.EncodeToString(publicBytes),
		"VAPID_PRIVATE_KEY":      base64.RawURLEncoding.EncodeToString(privateBytes),
	}
	values["VITE_VAPID_PUBLIC_KEY"] = values["VAPID_PUBLIC_KEY"]

	lines := strings.Split(string(template), "\n")
	for index, line := range lines {
		key, _, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		if value, ok := values[key]; ok {
			lines[index] = key + "=" + value
		}
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0700); err != nil {
		panic(fmt.Errorf("create output directory: %w", err))
	}
	file, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		panic(fmt.Errorf("create output: %w", err))
	}
	if _, err := file.WriteString(strings.Join(lines, "\n")); err != nil {
		file.Close()
		panic(fmt.Errorf("write output: %w", err))
	}
	if err := file.Close(); err != nil {
		panic(fmt.Errorf("close output: %w", err))
	}
	if err := os.Chmod(outputPath, 0600); err != nil {
		panic(fmt.Errorf("set output permissions: %w", err))
	}

	fmt.Printf("generated local secrets in %s (values not printed)\n", outputPath)
}
