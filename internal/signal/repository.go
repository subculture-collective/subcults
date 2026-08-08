package signal

import (
	"context"
	"sort"
	"sync"
)

// Repository is intentionally small so a durable adapter can preserve the same
// revision, snapshot, and delivery idempotency invariants.
type Repository interface {
	CreateSignal(context.Context, Signal, Revision) error
	GetSignal(context.Context, string) (Signal, error)
	GetLatestRevision(context.Context, string) (Revision, error)
	GetRevision(context.Context, string) (Revision, error)
	CreateRevision(context.Context, Revision) error
	UpdateSignalState(context.Context, string, State) error
	CreateDelivery(context.Context, Delivery) (Delivery, error)
	GetDelivery(context.Context, string) (Delivery, error)
	UpdateDelivery(context.Context, Delivery) error
}

type InMemoryRepository struct {
	mu           sync.RWMutex
	signals      map[string]Signal
	revisions    map[string]Revision
	revisionIDs  map[string][]string
	deliveries   map[string]Delivery
	deliveryKeys map[string]string
}

func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{signals: map[string]Signal{}, revisions: map[string]Revision{}, revisionIDs: map[string][]string{}, deliveries: map[string]Delivery{}, deliveryKeys: map[string]string{}}
}
func (r *InMemoryRepository) CreateSignal(ctx context.Context, signal Signal, revision Revision) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := signal.Validate(); err != nil {
		return err
	}
	if err := revision.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.signals[signal.ID]; ok {
		return ErrInvalidSignal
	}
	signal.ConsentScopeIDs = append([]string(nil), signal.ConsentScopeIDs...)
	r.signals[signal.ID] = signal
	r.revisions[revision.ID] = revision
	r.revisionIDs[signal.ID] = []string{revision.ID}
	return nil
}
func (r *InMemoryRepository) GetSignal(ctx context.Context, id string) (Signal, error) {
	if err := ctx.Err(); err != nil {
		return Signal{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.signals[id]
	if !ok {
		return Signal{}, ErrSignalNotFound
	}
	v.ConsentScopeIDs = append([]string(nil), v.ConsentScopeIDs...)
	return v, nil
}
func (r *InMemoryRepository) GetLatestRevision(ctx context.Context, signalID string) (Revision, error) {
	if err := ctx.Err(); err != nil {
		return Revision{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := r.revisionIDs[signalID]
	if len(ids) == 0 {
		return Revision{}, ErrRevisionNotFound
	}
	return r.revisions[ids[len(ids)-1]], nil
}
func (r *InMemoryRepository) GetRevision(ctx context.Context, id string) (Revision, error) {
	if err := ctx.Err(); err != nil {
		return Revision{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.revisions[id]
	if !ok {
		return Revision{}, ErrRevisionNotFound
	}
	return v, nil
}
func (r *InMemoryRepository) CreateRevision(ctx context.Context, revision Revision) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := revision.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.signals[revision.SignalID]; !ok {
		return ErrSignalNotFound
	}
	if _, ok := r.revisions[revision.ID]; ok {
		return ErrInvalidSignal
	}
	ids := r.revisionIDs[revision.SignalID]
	for _, id := range ids {
		if r.revisions[id].Number == revision.Number {
			return ErrInvalidSignal
		}
	}
	r.revisions[revision.ID] = revision
	r.revisionIDs[revision.SignalID] = append(ids, revision.ID)
	sort.Slice(r.revisionIDs[revision.SignalID], func(i, j int) bool {
		return r.revisions[r.revisionIDs[revision.SignalID][i]].Number < r.revisions[r.revisionIDs[revision.SignalID][j]].Number
	})
	return nil
}
func (r *InMemoryRepository) UpdateSignalState(ctx context.Context, id string, state State) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.signals[id]
	if !ok {
		return ErrSignalNotFound
	}
	v.State = state
	r.signals[id] = v
	return nil
}
func deliveryKey(d Delivery) string {
	return d.SignalRevisionID + "\x00" + d.ContactPointID + "\x00" + d.Channel
}
func (r *InMemoryRepository) CreateDelivery(ctx context.Context, d Delivery) (Delivery, error) {
	if err := ctx.Err(); err != nil {
		return Delivery{}, err
	}
	if err := d.Validate(); err != nil {
		return Delivery{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.revisions[d.SignalRevisionID]; !ok {
		return Delivery{}, ErrRevisionNotFound
	}
	key := deliveryKey(d)
	if id, ok := r.deliveryKeys[key]; ok {
		return r.deliveries[id], nil
	}
	r.deliveries[d.ID] = d
	r.deliveryKeys[key] = d.ID
	return d, nil
}
func (r *InMemoryRepository) GetDelivery(ctx context.Context, id string) (Delivery, error) {
	if err := ctx.Err(); err != nil {
		return Delivery{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.deliveries[id]
	if !ok {
		return Delivery{}, ErrDeliveryNotFound
	}
	return v, nil
}
func (r *InMemoryRepository) UpdateDelivery(ctx context.Context, d Delivery) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.deliveries[d.ID]; !ok {
		return ErrDeliveryNotFound
	}
	r.deliveries[d.ID] = d
	return nil
}
