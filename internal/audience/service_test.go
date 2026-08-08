package audience

import (
	"context"
	"testing"
	"time"
)

func TestCanDeliverConsentInvariants(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, time.August, 8, 12, 0, 0, 0, time.UTC)
	profileScope := DeliveryScope{
		SenderType:        "profile",
		SenderID:          "profile-a",
		Channel:           "email",
		Purpose:           "tour_updates",
		DisclosureVersion: "tour-updates-v1",
	}
	tourChicagoScope := DeliveryScope{
		SenderType:        "profile",
		SenderID:          "profile-a",
		Channel:           "email",
		Purpose:           "tour_updates",
		TourID:            "tour-a",
		PlaceID:           "chicago",
		DisclosureVersion: "tour-updates-v1",
	}
	tourDetroitScope := tourChicagoScope
	tourDetroitScope.PlaceID = "detroit"

	tests := []struct {
		name    string
		setup   func(t *testing.T, service *Service)
		request DeliveryScope
		want    bool
	}{
		{
			name: "rsvp does not authorize delivery",
			setup: func(t *testing.T, service *Service) {
				t.Helper()
				if err := service.RecordRelationship(ctx, Relationship{SubjectDID: "did:plc:a", ProgramType: "profile", ProgramID: "profile-a", Kind: "rsvp", OccurredAt: now}); err != nil {
					t.Fatalf("record rsvp: %v", err)
				}
			},
			request: profileScope,
			want:    false,
		},
		{
			name: "verified contact without grant cannot receive marketing",
			setup: func(t *testing.T, service *Service) {
				t.Helper()
				if _, err := service.CreateScope(ctx, profileScope); err != nil {
					t.Fatalf("create scope: %v", err)
				}
			},
			request: profileScope,
			want:    false,
		},
		{
			name: "profile wide grant contains tour place request",
			setup: func(t *testing.T, service *Service) {
				t.Helper()
				grant(t, service, profileScope, now)
			},
			request: tourChicagoScope,
			want:    true,
		},
		{
			name: "narrow revoke overrides profile wide grant",
			setup: func(t *testing.T, service *Service) {
				t.Helper()
				grant(t, service, profileScope, now)
				scopeID := createScope(t, service, tourChicagoScope)
				if err := service.Revoke(ctx, "contact-a", scopeID, "preference_center", map[string]string{"request_id": "revoke-1"}, now.Add(time.Minute)); err != nil {
					t.Fatalf("revoke: %v", err)
				}
			},
			request: tourChicagoScope,
			want:    false,
		},
		{
			name: "tour place grant does not match another place",
			setup: func(t *testing.T, service *Service) {
				t.Helper()
				grant(t, service, tourChicagoScope, now)
			},
			request: tourDetroitScope,
			want:    false,
		},
		{
			name: "global suppression overrides exact grant",
			setup: func(t *testing.T, service *Service) {
				t.Helper()
				grant(t, service, profileScope, now)
				if err := service.Suppress(ctx, Suppression{ContactPointID: "contact-a", Level: SuppressionGlobal, OccurredAt: now.Add(time.Minute)}); err != nil {
					t.Fatalf("suppress globally: %v", err)
				}
			},
			request: profileScope,
			want:    false,
		},
		{
			name: "scope suppression overrides profile wide grant",
			setup: func(t *testing.T, service *Service) {
				t.Helper()
				grant(t, service, profileScope, now)
				scopeID := createScope(t, service, tourChicagoScope)
				if err := service.Suppress(ctx, Suppression{ContactPointID: "contact-a", Level: SuppressionScope, ScopeID: scopeID, OccurredAt: now.Add(time.Minute)}); err != nil {
					t.Fatalf("suppress scope: %v", err)
				}
			},
			request: tourChicagoScope,
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := newVerifiedService(t, now)
			tt.setup(t, service)
			got, err := service.CanDeliver(ctx, "contact-a", tt.request)
			if err != nil {
				t.Fatalf("CanDeliver: %v", err)
			}
			if got != tt.want {
				t.Fatalf("CanDeliver = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCanDeliverRejectsUnverifiedContactEvenWithGrant(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	service := NewService(NewInMemoryRepository())
	if err := service.AddContact(ctx, ContactPoint{ID: "contact-a", Kind: "email"}); err != nil {
		t.Fatalf("add contact: %v", err)
	}
	scope := DeliveryScope{SenderType: "profile", SenderID: "profile-a", Channel: "email", Purpose: "tour_updates", DisclosureVersion: "tour-updates-v1"}
	grant(t, service, scope, now)

	allowed, err := service.CanDeliver(ctx, "contact-a", scope)
	if err != nil {
		t.Fatalf("CanDeliver: %v", err)
	}
	if allowed {
		t.Fatal("unverified contact must not be deliverable")
	}
}

func TestDIDContactLinkIsSeparateFromConsent(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	service := newVerifiedService(t, now)
	if err := service.LinkContact(ctx, ContactPointLink{ContactPointID: "contact-a", UserDID: "did:plc:a", VerifiedAt: now}); err != nil {
		t.Fatalf("link contact: %v", err)
	}
	if err := service.RecordRelationship(ctx, Relationship{SubjectDID: "did:plc:a", ProgramType: "profile", ProgramID: "profile-a", Kind: "membership", OccurredAt: now}); err != nil {
		t.Fatalf("record membership: %v", err)
	}

	scope := DeliveryScope{SenderType: "profile", SenderID: "profile-a", Channel: "push", Purpose: "marketing", DisclosureVersion: "marketing-v1"}
	allowed, err := service.CanDeliver(ctx, "contact-a", scope)
	if err != nil {
		t.Fatalf("CanDeliver: %v", err)
	}
	if allowed {
		t.Fatal("a DID-contact link and membership must not grant delivery consent")
	}
}

func TestActiveContactsForDIDRequiresCurrentProof(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	service := newVerifiedService(t, now)
	if err := service.LinkContact(ctx, ContactPointLink{ContactPointID: "contact-a", UserDID: "did:plc:a", VerifiedAt: now}); err != nil {
		t.Fatal(err)
	}
	contacts, err := service.ActiveContactsForDID(ctx, "did:plc:a")
	if err != nil || len(contacts) != 1 || contacts[0].ID != "contact-a" {
		t.Fatalf("contacts=%#v err=%v", contacts, err)
	}
}

func newVerifiedService(t *testing.T, verifiedAt time.Time) *Service {
	t.Helper()
	service := NewService(NewInMemoryRepository())
	if err := service.AddContact(context.Background(), ContactPoint{ID: "contact-a", Kind: "email", VerifiedAt: &verifiedAt}); err != nil {
		t.Fatalf("add verified contact: %v", err)
	}
	return service
}

func createScope(t *testing.T, service *Service, scope DeliveryScope) string {
	t.Helper()
	id, err := service.CreateScope(context.Background(), scope)
	if err != nil {
		t.Fatalf("create scope: %v", err)
	}
	return id
}

func grant(t *testing.T, service *Service, scope DeliveryScope, at time.Time) {
	t.Helper()
	scopeID := createScope(t, service, scope)
	if err := service.Grant(context.Background(), "contact-a", scopeID, "signup_form", map[string]string{"form": "tour-interest"}, at); err != nil {
		t.Fatalf("grant: %v", err)
	}
}

func TestConsentDoesNotCrossDisclosureVersions(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	service := newVerifiedService(t, now)
	v1 := DeliveryScope{SenderType: "profile", SenderID: "profile-a", Channel: "email", Purpose: "tour_updates", DisclosureVersion: "v1"}
	grant(t, service, v1, now)
	v2 := v1
	v2.DisclosureVersion = "v2"
	allowed, err := service.CanDeliver(ctx, "contact-a", v2)
	if err != nil {
		t.Fatalf("CanDeliver: %v", err)
	}
	if allowed {
		t.Fatal("a v1 disclosure grant must not authorize delivery under v2")
	}
}
