package notification

import (
	"context"
	"testing"

	"github.com/onnwee/subcults/internal/identity"
)

func TestSubscriptionIsEncryptedAndRevocable(t *testing.T) {
	protector, err := identity.NewEphemeralContactProtector()
	if err != nil {
		t.Fatal(err)
	}
	repository := NewInMemoryRepository()
	service := NewService(repository, protector, "", "", "")
	input := BrowserSubscription{Endpoint: "https://push.example.test/subscription/secret", Keys: Keys{P256DH: "public-key", Auth: "auth-secret"}}
	if err := service.Subscribe(context.Background(), "user-1", "test-agent", input); err != nil {
		t.Fatal(err)
	}
	items, err := repository.ListActive(context.Background(), "user-1")
	if err != nil || len(items) != 1 {
		t.Fatalf("active subscriptions: %d, %v", len(items), err)
	}
	if string(items[0].Endpoint) == input.Endpoint || string(items[0].Auth) == input.Keys.Auth {
		t.Fatal("subscription secret was stored in plaintext")
	}
	if err := service.Unsubscribe(context.Background(), "user-1", input.Endpoint); err != nil {
		t.Fatal(err)
	}
	items, _ = repository.ListActive(context.Background(), "user-1")
	if len(items) != 0 {
		t.Fatal("subscription remains active after revocation")
	}
}

func TestSubscriptionRejectsInsecureEndpoint(t *testing.T) {
	protector, _ := identity.NewEphemeralContactProtector()
	service := NewService(NewInMemoryRepository(), protector, "", "", "")
	err := service.Subscribe(context.Background(), "user-1", "agent", BrowserSubscription{Endpoint: "http://push.example.test", Keys: Keys{P256DH: "p", Auth: "a"}})
	if err == nil {
		t.Fatal("accepted insecure push endpoint")
	}
}
