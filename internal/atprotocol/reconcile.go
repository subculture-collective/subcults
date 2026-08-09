package atprotocol

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// PendingRecord is a canonical write waiting for its synchronization event.
type PendingRecord struct {
	RecordMapping
	HostURL string
}

// Reconciler fetches authoritative PDS records whose Tap event has not arrived
// within the publication latency boundary.
type Reconciler struct {
	store    *SQLStore
	client   *http.Client
	after    time.Duration
	interval time.Duration
}

func NewReconciler(store *SQLStore) *Reconciler {
	return &Reconciler{
		store:    store,
		client:   newAuthoritativePDSClient(),
		after:    15 * time.Second,
		interval: 5 * time.Second,
	}
}

// Run blocks until context cancellation and safely retries pending records.
func (r *Reconciler) Run(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = r.ReconcileOnce(ctx)
		}
	}
}

func (r *Reconciler) ReconcileOnce(ctx context.Context) error {
	pending, err := r.store.PendingRecords(ctx, time.Now().UTC().Add(-r.after), 100)
	if err != nil {
		return err
	}
	for _, mapping := range pending {
		if err := r.reconcileRecord(ctx, mapping); err != nil {
			continue
		}
	}
	return nil
}

func (r *Reconciler) reconcileRecord(ctx context.Context, mapping PendingRecord) error {
	base, err := url.Parse(mapping.HostURL)
	if err != nil || validateAuthoritativePDSURL(base) != nil {
		return fmt.Errorf("invalid authoritative PDS URL")
	}
	base.Path = "/xrpc/com.atproto.repo.getRecord"
	base.RawPath = ""
	base.RawQuery = ""
	base.Fragment = ""
	query := base.Query()
	query.Set("repo", mapping.PublisherDID)
	query.Set("collection", mapping.Collection)
	query.Set("rkey", mapping.RKey)
	base.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base.String(), nil)
	if err != nil {
		return err
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("authoritative PDS returned HTTP %d", resp.StatusCode)
	}
	var result struct {
		URI   string          `json:"uri"`
		CID   string          `json:"cid"`
		Value json.RawMessage `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}
	if result.URI != mapping.ATURI || result.CID == "" {
		return ErrRecordConflict
	}
	_, err = r.store.IngestObservation(ctx, "reconciliation", 0, TapRecordEvent{
		Rev: result.CID, DID: mapping.PublisherDID, Collection: mapping.Collection, RKey: mapping.RKey,
		Action: "update", CID: result.CID, Record: result.Value,
	}, time.Now().UTC())
	return err
}

func (s *SQLStore) PendingRecords(ctx context.Context, before time.Time, limit int) ([]PendingRecord, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT m.entity_type,m.entity_id::text,m.publisher_did,m.collection,m.rkey,m.at_uri,
		COALESCE(m.cid,''),m.projection_status,m.record_version,m.updated_at,l.host_url
		FROM atproto_record_mappings m JOIN atproto_oauth_links l ON l.account_did=m.publisher_did
		WHERE m.projection_status IN ('reserved','awaiting_projection') AND m.updated_at <= $1
		ORDER BY m.updated_at LIMIT $2`, before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []PendingRecord{}
	for rows.Next() {
		var item PendingRecord
		if err := rows.Scan(&item.EntityType, &item.EntityID, &item.PublisherDID, &item.Collection, &item.RKey, &item.ATURI, &item.CID, &item.ProjectionStatus, &item.RecordVersion, &item.UpdatedAt, &item.HostURL); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func newAuthoritativePDSClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, fmt.Errorf("invalid PDS address: %w", err)
		}
		addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("resolve PDS host: %w", err)
		}
		if len(addresses) == 0 {
			return nil, fmt.Errorf("PDS host has no addresses")
		}
		for _, address := range addresses {
			if !isPublicInternetIP(address.IP) {
				return nil, fmt.Errorf("PDS host resolves to a non-public address")
			}
		}
		dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
		return dialer.DialContext(ctx, network, net.JoinHostPort(addresses[0].IP.String(), port))
	}
	return &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func validateAuthoritativePDSURL(endpoint *url.URL) error {
	if endpoint == nil || endpoint.Scheme != "https" || endpoint.Host == "" || endpoint.User != nil {
		return fmt.Errorf("authoritative PDS must be an HTTPS origin without credentials")
	}
	host := strings.TrimSpace(endpoint.Hostname())
	if host == "" || strings.EqualFold(host, "localhost") {
		return fmt.Errorf("authoritative PDS host is not public")
	}
	if parsed := net.ParseIP(host); parsed != nil && !isPublicInternetIP(parsed) {
		return fmt.Errorf("authoritative PDS host is not public")
	}
	return nil
}

func isPublicInternetIP(ip net.IP) bool {
	return ip != nil && !ip.IsLoopback() && !ip.IsPrivate() && !ip.IsLinkLocalUnicast() &&
		!ip.IsLinkLocalMulticast() && !ip.IsUnspecified() && !ip.IsMulticast()
}
