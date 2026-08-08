package signal

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/onnwee/subcults/internal/audience"
)

type Dispatcher struct {
	repository Repository
	audience   interface {
		CanDeliver(context.Context, string, audience.DeliveryScope) (bool, error)
	}
	providers map[string]Provider
	now       func() time.Time
}

func NewDispatcher(repository Repository, audienceService *audience.Service, providers map[string]Provider) *Dispatcher {
	return &Dispatcher{repository: repository, audience: audienceService, providers: providers, now: time.Now}
}

// Dispatch rechecks the consent ledger at the irreversible edge: immediately before Send.
func (d *Dispatcher) Dispatch(ctx context.Context, deliveryID string) error {
	delivery, err := d.repository.GetDelivery(ctx, deliveryID)
	if err != nil {
		return err
	}
	if delivery.State == DeliverySent || delivery.State == DeliveryDelivered {
		return nil
	}
	allowed, err := d.audience.CanDeliver(ctx, delivery.ContactPointID, delivery.Scope)
	if err != nil {
		return err
	}
	if !allowed {
		delivery.State = DeliverySuppressed
		delivery.UpdatedAt = d.now()
		if err := d.repository.UpdateDelivery(ctx, delivery); err != nil {
			return err
		}
		return ErrSuppressed
	}
	provider, ok := d.providers[delivery.Provider]
	if !ok {
		delivery.State = DeliveryFailed
		delivery.UpdatedAt = d.now()
		_ = d.repository.UpdateDelivery(ctx, delivery)
		return fmt.Errorf("%w: %s", ErrProvider, delivery.Provider)
	}
	revision, err := d.repository.GetRevision(ctx, delivery.SignalRevisionID)
	if err != nil {
		return err
	}
	messageID, err := provider.Send(ctx, Message{Channel: delivery.Channel, ToToken: delivery.ToToken, Subject: revision.Content.Subject, Body: revision.Content.Body, DeepLink: revision.Content.DeepLink})
	if err != nil {
		delivery.State = DeliveryFailed
		delivery.UpdatedAt = d.now()
		_ = d.repository.UpdateDelivery(ctx, delivery)
		return fmt.Errorf("%w: %v", ErrProvider, err)
	}
	delivery.State = DeliverySent
	delivery.ProviderMessageID = messageID
	delivery.UpdatedAt = d.now()
	if err := d.repository.UpdateDelivery(ctx, delivery); err != nil {
		return err
	}
	return nil
}
func IsSuppressed(err error) bool { return errors.Is(err, ErrSuppressed) }
