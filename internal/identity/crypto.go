package identity

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
)

type ContactProtector struct {
	aead    cipher.AEAD
	hmacKey []byte
}

func NewContactProtector(encryptionKey, hmacKey []byte) (*ContactProtector, error) {
	if len(encryptionKey) != 32 {
		return nil, errors.New("contact encryption key must be 32 bytes")
	}
	if len(hmacKey) < 32 {
		return nil, errors.New("contact HMAC key must be at least 32 bytes")
	}
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("create contact cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create contact AEAD: %w", err)
	}
	return &ContactProtector{aead: aead, hmacKey: append([]byte(nil), hmacKey...)}, nil
}

func NewContactProtectorFromBase64(encryptionKey, hmacKey string) (*ContactProtector, error) {
	enc, err := base64.StdEncoding.DecodeString(encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("decode contact encryption key: %w", err)
	}
	mac, err := base64.StdEncoding.DecodeString(hmacKey)
	if err != nil {
		return nil, fmt.Errorf("decode contact HMAC key: %w", err)
	}
	return NewContactProtector(enc, mac)
}

func NewEphemeralContactProtector() (*ContactProtector, error) {
	encryptionKey := make([]byte, 32)
	hmacKey := make([]byte, 32)
	if _, err := rand.Read(encryptionKey); err != nil {
		return nil, fmt.Errorf("generate ephemeral encryption key: %w", err)
	}
	if _, err := rand.Read(hmacKey); err != nil {
		return nil, fmt.Errorf("generate ephemeral HMAC key: %w", err)
	}
	return NewContactProtector(encryptionKey, hmacKey)
}

func (p *ContactProtector) Protect(value string) ([]byte, string, error) {
	nonce := make([]byte, p.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, "", fmt.Errorf("generate contact nonce: %w", err)
	}
	ciphertext := p.aead.Seal(nonce, nonce, []byte(value), nil)
	mac := hmac.New(sha256.New, p.hmacKey)
	_, _ = mac.Write([]byte(value))
	return ciphertext, hex.EncodeToString(mac.Sum(nil)), nil
}

func (p *ContactProtector) Reveal(ciphertext []byte) (string, error) {
	if len(ciphertext) < p.aead.NonceSize() {
		return "", errors.New("contact ciphertext is truncated")
	}
	nonce := ciphertext[:p.aead.NonceSize()]
	plaintext, err := p.aead.Open(nil, nonce, ciphertext[p.aead.NonceSize():], nil)
	if err != nil {
		return "", errors.New("decrypt contact value")
	}
	return string(plaintext), nil
}

func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
