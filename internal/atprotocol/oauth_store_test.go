package atprotocol

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestSessionCipher(t *testing.T) {
	key := base64.StdEncoding.EncodeToString(make([]byte, 32))
	cipher, err := NewSessionCipherFromBase64(key)
	if err != nil {
		t.Fatal(err)
	}
	value := map[string]string{"refresh_token": "never-plaintext"}
	encrypted, err := cipher.encrypt(value)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encrypted), "never-plaintext") {
		t.Fatal("ciphertext contains secret plaintext")
	}
	var decoded map[string]string
	if err := cipher.decrypt(encrypted, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["refresh_token"] != value["refresh_token"] {
		t.Fatalf("round trip changed value: %+v", decoded)
	}
}

func TestSessionCipherRejectsWrongSize(t *testing.T) {
	_, err := NewSessionCipherFromBase64(base64.StdEncoding.EncodeToString(make([]byte, 31)))
	if err == nil {
		t.Fatal("expected key-size validation")
	}
}
