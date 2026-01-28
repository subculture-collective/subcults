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

The Jetstream indexer exposes metrics at `/internal/indexer/metrics` (requires authentication).

#### `indexer_messages_processed_total`

**Type**: Counter  
**Description**: Total number of messages successfully processed from Jetstream.

**Usage**: Track overall indexer throughput and message volume.

**Example Queries**:
```promql
# Messages processed per second
rate(indexer_messages_processed_total[1m])

# Total messages processed in the last hour
increase(indexer_messages_processed_total[1h])
```

#### `indexer_messages_error_total`

**Type**: Counter  
**Description**: Total number of messages that resulted in processing errors.

**Usage**: Monitor indexer reliability and identify issues.

**Example Queries**:
```promql
# Error rate per second
rate(indexer_messages_error_total[1m])

# Error percentage
rate(indexer_messages_error_total[5m]) / rate(indexer_messages_processed_total[5m]) * 100
```

#### `indexer_upserts_total`

**Type**: Counter  
**Description**: Total number of database upsert operations performed.

**Usage**: Track database write activity.

#### `indexer_trust_recompute_total`

**Type**: Counter  
**Description**: Total number of trust score recomputation operations triggered.

**Usage**: Monitor trust graph computation frequency.

#### `indexer_ingest_latency_seconds`

**Type**: Histogram  
**Description**: Time taken to process individual messages (from receipt to completion).

**Buckets**: Default Prometheus buckets

**Usage**: Monitor message processing performance.

**Example Queries**:
```promql
# 95th percentile processing latency
histogram_quantile(0.95, rate(indexer_ingest_latency_seconds_bucket[5m]))

# Average processing latency
rate(indexer_ingest_latency_seconds_sum[5m]) / rate(indexer_ingest_latency_seconds_count[5m])
```

#### `indexer_processing_lag_seconds`

**Type**: Gauge  
**Description**: Time difference between message timestamp and processing time (indicates how far behind real-time the indexer is).

**Usage**: Monitor indexer lag and detect backlog issues.

**Example Queries**:
```promql
# Current processing lag
indexer_processing_lag_seconds

# Maximum lag in the last 5 minutes
max_over_time(indexer_processing_lag_seconds[5m])
```

#### `indexer_reconnection_attempts_total`

**Type**: Counter  
**Description**: Total number of reconnection attempts to Jetstream.

**Usage**: Monitor connection stability and network issues.

**Example Queries**:
```promql
# Reconnection rate per minute
rate(indexer_reconnection_attempts_total[1m]) * 60

# Total reconnections in the last hour
increase(indexer_reconnection_attempts_total[1h])
```

#### `indexer_database_writes_failed_total`

**Type**: Counter  
**Description**: Total number of failed database write operations.

**Usage**: Monitor database health and identify persistence issues.

**Example Queries**:
```promql
# Database failure rate
rate(indexer_database_writes_failed_total[5m])

# Failure percentage
rate(indexer_database_writes_failed_total[5m]) / rate(indexer_upserts_total[5m]) * 100
```

#### `indexer_backpressure_paused_total`

**Type**: Counter  
**Description**: Total number of times message consumption was paused due to backpressure.

**Usage**: Monitor queue saturation and processing capacity.

#### `indexer_backpressure_resumed_total`

**Type**: Counter  
**Description**: Total number of times message consumption resumed after backpressure.

**Usage**: Track backpressure recovery cycles.

#### `indexer_backpressure_pause_duration_seconds`

**Type**: Histogram  
**Description**: Duration of backpressure pause events.

**Buckets**: 0.1, 0.5, 1, 5, 10, 30, 60 seconds

**Usage**: Analyze backpressure severity and duration.

**Example Queries**:
```promql
# 95th percentile pause duration
histogram_quantile(0.95, rate(indexer_backpressure_pause_duration_seconds_bucket[5m]))

# Average pause duration
rate(indexer_backpressure_pause_duration_seconds_sum[5m]) / rate(indexer_backpressure_pause_duration_seconds_count[5m])
```

#### `indexer_pending_messages`

**Type**: Gauge  
**Description**: Current number of messages waiting in the processing queue.

**Usage**: Monitor queue depth and detect processing bottlenecks.

**Example Queries**:
```promql
# Current queue depth
indexer_pending_messages

# Maximum queue depth in the last hour
max_over_time(indexer_pending_messages[1h])
```

### Indexer Alert Conditions

Recommended Prometheus alert rules for the indexer:

```yaml
groups:
  - name: indexer
    rules:
      # High processing lag
      - alert: HighIndexerProcessingLag
        expr: indexer_processing_lag_seconds > 60
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Indexer is lagging behind real-time"
          description: "Processing lag is {{ $value }}s (threshold: 60s)"

      # Frequent reconnections
      - alert: FrequentIndexerReconnections
        expr: rate(indexer_reconnection_attempts_total[5m]) > 0.1
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "Indexer experiencing frequent reconnections"
          description: "Reconnection rate is {{ $value }}/sec"

      # High error rate
      - alert: HighIndexerErrorRate
        expr: rate(indexer_messages_error_total[5m]) / rate(indexer_messages_processed_total[5m]) > 0.05
        for: 10m
        labels:
          severity: critical
        annotations:
          summary: "High indexer error rate"
          description: "Error rate is {{ $value | humanizePercentage }}"

      # Database write failures
      - alert: IndexerDatabaseWriteFailures
        expr: rate(indexer_database_writes_failed_total[5m]) > 0
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "Indexer database writes failing"
          description: "Database write failure rate is {{ $value }}/sec"

      # Prolonged backpressure
      - alert: IndexerBackpressure
        expr: indexer_pending_messages > 1000
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Indexer experiencing backpressure"
          description: "Pending messages: {{ $value }} (threshold: 1000)"
```

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
        expr: histogram_quantile(0.95, rate(stream_join_latency_seconds_bucket[5m])) > 3
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "Stream join latency above 3s (p95)"
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

  - job_name: 'subcults-indexer'
    static_configs:
      - targets: ['indexer:9090']
    scrape_interval: 15s
    metrics_path: /internal/indexer/metrics
    # Optional: Add authentication if configured
    # bearer_token: '<internal-auth-token>'
```

### Grafana Dashboards

#### Streaming Dashboard

Recommended panels:

1. **Join Rate Timeline**: `rate(stream_joins_total[5m])`
2. **Leave Rate Timeline**: `rate(stream_leaves_total[5m])`
3. **Join Latency Heatmap**: Histogram visualization of `stream_join_latency_seconds`
4. **P95 Latency**: `histogram_quantile(0.95, rate(stream_join_latency_seconds_bucket[5m]))`
5. **Active Sessions**: Count from database query

#### Indexer Dashboard

Recommended panels:

1. **Message Processing Rate**: `rate(indexer_messages_processed_total[1m])`
2. **Error Rate**: `rate(indexer_messages_error_total[1m])`
3. **Processing Lag**: `indexer_processing_lag_seconds`
4. **Queue Depth**: `indexer_pending_messages`
5. **Reconnection Rate**: `rate(indexer_reconnection_attempts_total[5m])`
6. **P95 Ingest Latency**: `histogram_quantile(0.95, rate(indexer_ingest_latency_seconds_bucket[5m]))`
7. **Backpressure Events**: `rate(indexer_backpressure_paused_total[5m])`
8. **Database Write Failures**: `rate(indexer_database_writes_failed_total[5m])`

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
