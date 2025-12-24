# Observability Guide

## Overview

Subcults implements comprehensive observability for operational insight, performance monitoring, and troubleshooting. This guide covers the available metrics, their usage, and privacy considerations.

## Metrics Endpoint

All metrics are exposed via the Prometheus exposition format at:

```
GET /metrics
```

The endpoint returns metrics in text format compatible with Prometheus scraping.

## Available Metrics

### Stream Session Metrics

#### `stream_joins_total`

**Type**: Counter  
**Description**: Total number of stream join events across all sessions.

**Usage**: Track overall streaming participation and popularity trends.

**Example Queries**:
```promql
# Total joins in the last hour
rate(stream_joins_total[1h]) * 3600

# Join rate per minute
rate(stream_joins_total[1m]) * 60
```

#### `stream_leaves_total`

**Type**: Counter  
**Description**: Total number of stream leave events across all sessions.

**Usage**: Track user engagement duration and churn patterns.

**Example Queries**:
```promql
# Total leaves in the last hour
rate(stream_leaves_total[1h]) * 3600

# Leave rate per minute
rate(stream_leaves_total[1m]) * 60

# Join to leave ratio (retention indicator)
stream_joins_total / stream_leaves_total
```

#### `stream_join_latency_seconds`

**Type**: Histogram  
**Description**: Time from token issuance to successful stream join completion (first audio track subscription).

**Buckets**: 0.1, 0.25, 0.5, 1.0, 2.0, 5.0, 10.0, 30.0, 60.0 seconds

**Usage**: Monitor stream join performance and identify latency issues.

**Example Queries**:
```promql
# 95th percentile join latency
histogram_quantile(0.95, rate(stream_join_latency_seconds_bucket[5m]))

# 50th percentile (median) join latency
histogram_quantile(0.50, rate(stream_join_latency_seconds_bucket[5m]))

# Average join latency
rate(stream_join_latency_seconds_sum[5m]) / rate(stream_join_latency_seconds_count[5m])

# Percentage of joins completing within 2 seconds
sum(rate(stream_join_latency_seconds_bucket{le="2.0"}[5m])) / sum(rate(stream_join_latency_seconds_count[5m])) * 100
```

### Indexer Metrics

(Documented in indexer package - see `internal/indexer/metrics.go`)

## Recording Join/Leave Events

### Join Event

**Endpoint**: `POST /streams/{id}/join`

**Request Body** (optional):
```json
{
  "token_issued_at": "2025-12-24T00:44:43Z"
}
```

**Behavior**:
- Increments `stream_joins_total` counter
- Increments per-session `join_count` in database
- If `token_issued_at` provided, records join latency histogram sample
- Creates audit log entry (action: "joined")

**Response**:
```json
{
  "stream_id": "uuid-here",
  "room_name": "scene-123-1703376283",
  "join_count": 5,
  "status": "joined"
}
```

### Leave Event

**Endpoint**: `POST /streams/{id}/leave`

**Request Body**: None required

**Behavior**:
- Increments `stream_leaves_total` counter
- Increments per-session `leave_count` in database
- Creates audit log entry (action: "left")

**Response**:
```json
{
  "stream_id": "uuid-here",
  "room_name": "scene-123-1703376283",
  "leave_count": 3,
  "status": "left"
}
```

## Privacy Considerations

### No PII in Metrics Labels

All streaming metrics follow strict privacy guidelines:

- **DO NOT** include user DIDs in metric labels
- **DO NOT** include participant identities in metric labels
- **DO NOT** include geographic coordinates in metric labels

Only use:
- Session IDs (UUIDs)
- Room names (anonymous identifiers)
- Aggregate counts

### Audit Logging

Join/leave events create audit log entries that include:
- User DID (hashed for tamper detection)
- Entity type: `"stream_session"`
- Entity ID: stream session UUID
- Action: `"joined"` or `"left"`
- Request ID for correlation

**Access to audit logs is restricted and requires appropriate authorization.**

## Performance Budgets

### Stream Join Latency Targets

- **p50** (median): < 1.0 second
- **p95**: < 2.0 seconds
- **p99**: < 5.0 seconds

### Alert Conditions

Recommended Prometheus alert rules:

```yaml
groups:
  - name: streaming
    rules:
      # High join latency
      - alert: HighStreamJoinLatency
        expr: histogram_quantile(0.95, rate(stream_join_latency_seconds_bucket[5m])) > 5
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "Stream join latency above 5s (p95)"
          description: "95th percentile join latency is {{ $value }}s"

      # Join failures (inferred from high leave rate shortly after joins)
      - alert: HighStreamChurnRate
        expr: rate(stream_leaves_total[5m]) / rate(stream_joins_total[5m]) > 0.8
        for: 15m
        labels:
          severity: warning
        annotations:
          summary: "High stream churn rate detected"
          description: "{{ $value | humanizePercentage }} of users leaving shortly after joining"
```

## Historical Analytics

Per-session aggregate counts (`join_count`, `leave_count`) are stored in the `stream_sessions` table for historical queries:

```sql
-- Average joins per session
SELECT AVG(join_count) FROM stream_sessions WHERE ended_at IS NOT NULL;

-- Sessions with high churn (more leaves than joins)
SELECT id, room_name, join_count, leave_count
FROM stream_sessions
WHERE leave_count > join_count
  AND ended_at IS NOT NULL;

-- Total participation over time
SELECT DATE_TRUNC('day', started_at) as day,
       SUM(join_count) as total_joins,
       SUM(leave_count) as total_leaves
FROM stream_sessions
WHERE started_at >= NOW() - INTERVAL '30 days'
GROUP BY day
ORDER BY day;
```

## Integration with Monitoring Stack

### Prometheus Configuration

Add scrape config to `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'subcults-api'
    static_configs:
      - targets: ['api:8080']
    scrape_interval: 15s
    metrics_path: /metrics
```

### Grafana Dashboards

Recommended panels:

1. **Join Rate Timeline**: `rate(stream_joins_total[5m])`
2. **Leave Rate Timeline**: `rate(stream_leaves_total[5m])`
3. **Join Latency Heatmap**: Histogram visualization of `stream_join_latency_seconds`
4. **P95 Latency**: `histogram_quantile(0.95, rate(stream_join_latency_seconds_bucket[5m]))`
5. **Active Sessions**: Count from database query

## Troubleshooting

### High Join Latency

Possible causes:
1. Network congestion between client and LiveKit SFU
2. TURN server overload (check relay usage)
3. Client device performance issues
4. WebRTC negotiation failures

**Investigation**:
- Check LiveKit server metrics
- Review client-side logs for ICE connection failures
- Verify TURN server availability

### Metric Discrepancies

If `stream_joins_total` and database `SUM(join_count)` don't match:
- Counter may have been reset (server restart)
- Some joins may have failed before database update
- Check audit logs for errors

**Resolution**: Use database counts as source of truth for historical data.

## Future Enhancements

Planned additions:
- Per-scene join/leave metrics (with scene ID label)
- Stream duration histogram
- Participant count gauge per active session
- Audio quality metrics integration
- Client-side error rate tracking

## See Also

- [API Reference](./API_REFERENCE.md) - Full API documentation
- [Privacy Guide](./PRIVACY.md) - Privacy policies and data handling
- [Architecture](./ARCHITECTURE.md) - System architecture overview
