package signal

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/onnwee/subcults/internal/audience"
)

type providerStub struct {
	calls int
	err   error
}

func (p *providerStub) Send(_ context.Context, _ Message) (string, error) {
	p.calls++
	if p.err != nil {
		return "", p.err
	}
	return "provider-1", nil
}

func fixture(t *testing.T) (*Service, *Dispatcher, *audience.Service, *InMemoryRepository, string, audience.DeliveryScope) {
	t.Helper()
	ctx := context.Background()
	repo := NewInMemoryRepository()
	audienceSvc := audience.NewService(audience.NewInMemoryRepository())
	now := time.Now()
	contactID := "contact-1"
	if err := audienceSvc.AddContact(ctx, audience.ContactPoint{ID: contactID, Kind: "email", VerifiedAt: &now}); err != nil {
		t.Fatal(err)
	}
	scope := audience.DeliveryScope{SenderType: "profile", SenderID: "profile-1", Channel: "email", Purpose: "tour_updates", DisclosureVersion: "2026-08"}
	scopeID, err := audienceSvc.CreateScope(ctx, scope)
	if err != nil {
		t.Fatal(err)
	}
	if err = audienceSvc.Grant(ctx, contactID, scopeID, "signal-test", map[string]string{"disclosure": "2026-08"}, now); err != nil {
		t.Fatal(err)
	}
	svc := NewService(repo)
	return svc, NewDispatcher(repo, audienceSvc, map[string]Provider{"mail": &providerStub{}}), audienceSvc, repo, contactID, scope
}
func draft() Signal {
	return Signal{ID: "signal-1", OwnerType: "profile", OwnerID: "profile-1", TargetType: "tour", TargetID: "tour-1"}
}
func TestPublishedSignalMaterialChangeCreatesRevision(t *testing.T) {
	ctx := context.Background()
	svc, _, _, _, _, _ := fixture(t)
	if _, err := svc.CreateDraft(ctx, draft(), Content{Subject: "Now", Body: "Chicago"}, `{"segment":"tour-city:detroit"}`, "did:owner", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Publish(ctx, "signal-1"); err != nil {
		t.Fatal(err)
	}
	rev, err := svc.ChangeAudience(ctx, "signal-1", `{"segment":"tour-city:chicago"}`, "did:owner")
	if err != nil {
		t.Fatal(err)
	}
	if rev.Number != 2 || rev.Supersedes == nil {
		t.Fatalf("revision=%+v", rev)
	}
}

func TestPublishedContentChangeCreatesRevision(t *testing.T) {
	ctx := context.Background()
	svc, _, _, _, _, _ := fixture(t)
	if _, err := svc.CreateDraft(ctx, draft(), Content{Body: "Chicago"}, `{"segment":"all"}`, "did:owner", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Publish(ctx, "signal-1"); err != nil {
		t.Fatal(err)
	}
	revision, err := svc.ChangeContent(ctx, "signal-1", Content{Body: "Chicago, doors at eight"}, "did:owner")
	if err != nil || revision.Number != 2 || revision.Supersedes == nil {
		t.Fatalf("revision=%+v err=%v", revision, err)
	}
}
func TestSnapshotIsDeterministicAndIdempotent(t *testing.T) {
	ctx := context.Background()
	svc, _, _, _, _, scope := fixture(t)
	rev, err := svc.CreateDraft(ctx, draft(), Content{Body: "Chicago"}, `{"segment":"all"}`, "did:owner", nil)
	if err != nil {
		t.Fatal(err)
	}
	first, err := svc.SnapshotDeliveries(ctx, rev.ID, "email", "tour_updates", "mail", scope, []AudienceMember{{ContactPointID: "b"}, {ContactPointID: "a"}}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.SnapshotDeliveries(ctx, rev.ID, "email", "tour_updates", "mail", scope, []AudienceMember{{ContactPointID: "a"}, {ContactPointID: "b"}}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if first[0].ContactPointID != "a" || first[0].ID != second[0].ID {
		t.Fatalf("first=%+v second=%+v", first, second)
	}
}
func TestDispatcherRechecksConsentImmediatelyBeforeSend(t *testing.T) {
	ctx := context.Background()
	svc, dispatcher, audienceSvc, repo, contactID, scope := fixture(t)
	rev, err := svc.CreateDraft(ctx, draft(), Content{Body: "Chicago"}, `{"segment":"all"}`, "did:owner", nil)
	if err != nil {
		t.Fatal(err)
	}
	deliveries, err := svc.SnapshotDeliveries(ctx, rev.ID, "email", "tour_updates", "mail", scope, []AudienceMember{{ContactPointID: contactID, ToToken: []byte("secret")}}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	scopeID, err := audienceSvc.ScopeIDFor(ctx, scope)
	if err != nil {
		t.Fatal(err)
	}
	if err := audienceSvc.Revoke(ctx, contactID, scopeID, "signal-test", map[string]string{"reason": "user request"}, time.Now()); err != nil {
		t.Fatal(err)
	}
	err = dispatcher.Dispatch(ctx, deliveries[0].ID)
	if !errors.Is(err, ErrSuppressed) {
		t.Fatalf("err=%v", err)
	}
	stored, err := repo.GetDelivery(ctx, deliveries[0].ID)
	if err != nil || stored.State != DeliverySuppressed {
		t.Fatalf("delivery=%+v err=%v", stored, err)
	}
}
func TestProviderFailureIsNotSuppression(t *testing.T) {
	ctx := context.Background()
	svc, _, audienceSvc, repo, contactID, scope := fixture(t)
	rev, _ := svc.CreateDraft(ctx, draft(), Content{Body: "Chicago"}, `{"segment":"all"}`, "did:owner", nil)
	deliveries, _ := svc.SnapshotDeliveries(ctx, rev.ID, "email", "tour_updates", "broken", scope, []AudienceMember{{ContactPointID: contactID}}, time.Now())
	dispatcher := NewDispatcher(repo, audienceSvc, map[string]Provider{})
	err := dispatcher.Dispatch(ctx, deliveries[0].ID)
	if !errors.Is(err, ErrProvider) || errors.Is(err, ErrSuppressed) {
		t.Fatalf("err=%v", err)
	}
}
