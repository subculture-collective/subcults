# Operations Guide

This document provides operational guidance for running and maintaining the Subcults platform in production.

## Table of Contents
- [Trust Score Recomputation](#trust-score-recomputation)
- [Performance SLOs](#performance-slos)
- [Monitoring & Alerting](#monitoring--alerting)
- [Configuration](#configuration)

---

## Trust Score Recomputation

### Overview

The trust recompute job periodically recalculates trust scores for scenes based on membership and alliance relationships. This job runs continuously in the background and processes scenes marked as "dirty" (requiring updates).

### Configuration

The trust recompute job supports the following configuration options:

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `TRUST_RECOMPUTE_TIMEOUT_SEC` | `30` | Maximum duration (in seconds) for a single recompute cycle. If exceeded, the job aborts and increments the error counter. **Note:** This environment variable is documented for future implementation; the timeout is currently configured via `RecomputeJobConfig.Timeout` in code. |

**Example:**
```bash
# Set timeout to 60 seconds for larger deployments
export TRUST_RECOMPUTE_TIMEOUT_SEC=60
```

### Metrics

The following Prometheus metrics are exposed at `/metrics` after the first recompute run:

**Note:** These metrics must be registered with the Prometheus registry during application startup. The `trust.Metrics` instance should be created and registered via `metrics.Register(prometheus.DefaultRegisterer)` in the service initialization code (typically in `cmd/api/main.go` or similar).

#### Counters
- **`trust_recompute_total`** - Total number of trust score recomputation operations completed
- **`trust_recompute_errors_total`** - Total number of trust score recomputation errors (including timeouts)

#### Histogram
- **`trust_recompute_duration_seconds`** - Histogram of trust score recomputation duration in seconds
  - Buckets: `[0.1, 0.25, 0.5, 1.0, 2.0, 5.0, 10.0]`

#### Gauges
- **`trust_last_recompute_timestamp`** - Unix timestamp of the last successful recompute operation
- **`trust_last_recompute_scene_count`** - Number of scenes processed in the last recompute operation

### Logging

The trust recompute job emits structured logs at the following levels:

#### INFO Level
- **Recompute Start**: Logged when a recompute cycle begins
  ```json
  {
    "level": "INFO",
    "msg": "recomputing trust scores",
    "dirty_count": 150
  }
  ```

- **Recompute Completion**: Logged when a recompute cycle completes successfully
  ```json
  {
    "level": "INFO",
    "msg": "trust recompute completed",
    "duration_seconds": 1.234,
    "scenes_processed": 150,
    "scenes_failed": 0,
    "avg_weight_variance": 0.05
  }
  ```

#### DEBUG Level
- **Batch Progress**: Logged every 10 scenes during processing
  ```json
  {
    "level": "DEBUG",
    "msg": "recompute progress",
    "processed": 20,
    "total": 150
  }
  ```

- **Scene Recompute**: Logged for each scene processed
  ```json
  {
    "level": "DEBUG",
    "msg": "trust score recomputed",
    "scene_id": "scene-123",
    "score": 0.75,
    "memberships": 5,
    "alliances": 3
  }
  ```

#### ERROR Level
- **Timeout**: Logged when the recompute timeout is exceeded
  ```json
  {
    "level": "ERROR",
    "msg": "trust recompute timeout exceeded",
    "processed": 75,
    "total": 150,
    "timeout": "30s"
  }
  ```

- **Scene Failure**: Logged when processing a specific scene fails
  ```json
  {
    "level": "ERROR",
    "msg": "failed to recompute trust score",
    "scene_id": "scene-123",
    "error": "database connection lost"
  }
  ```

---

## Performance SLOs

### Trust Recomputation Performance

| Metric | Target (SLO) | Critical Threshold | Notes |
|--------|-------------|-------------------|-------|
| **p95 Duration** | < 2s | < 5s | For deployments with < 10,000 scenes |
| **p99 Duration** | < 5s | < 10s | For deployments with < 10,000 scenes |
| **Error Rate** | < 0.1% | < 1% | Errors include timeouts and database failures |
| **Throughput** | > 100 scenes/sec | > 50 scenes/sec | Minimum acceptable processing rate |

### Scaling Guidance

As your deployment grows, monitor the following indicators:

- **10k+ scenes**: Consider increasing `TRUST_RECOMPUTE_TIMEOUT_SEC` to 60s and reviewing database indexes
- **50k+ scenes**: Consider implementing batch processing or sharding by scene geography
- **100k+ scenes**: Evaluate moving to a distributed job queue (e.g., Redis-backed worker pool)

---

## Monitoring & Alerting

### Recommended Alerts

Configure the following Prometheus alerts for production monitoring:

#### High-Priority Alerts

**Trust Recompute High Latency**
```yaml
alert: TrustRecomputeHighLatency
expr: histogram_quantile(0.95, rate(trust_recompute_duration_seconds_bucket[5m])) > 2
for: 5m
severity: warning
description: "Trust recompute p95 latency ({{ $value }}s) exceeds SLO target of 2s"
```

**Trust Recompute Critical Latency**
```yaml
alert: TrustRecomputeCriticalLatency
expr: histogram_quantile(0.95, rate(trust_recompute_duration_seconds_bucket[5m])) > 5
for: 5m
severity: critical
description: "Trust recompute p95 latency ({{ $value }}s) exceeds critical threshold of 5s"
```

**Trust Recompute High Error Rate**
```yaml
alert: TrustRecomputeHighErrorRate
expr: rate(trust_recompute_errors_total[5m]) / clamp_min(rate(trust_recompute_total[5m]), 1e-6) > 0.01
for: 5m
severity: critical
description: "Trust recompute error rate ({{ $value | humanizePercentage }}) exceeds 1%"
```

#### Medium-Priority Alerts

**Trust Recompute Stale**
```yaml
alert: TrustRecomputeStale
expr: time() - trust_last_recompute_timestamp > 300
for: 5m
severity: warning
description: "Trust recompute hasn't run successfully in {{ $value | humanizeDuration }}"
```

**Trust Recompute Processing Large Batch**
```yaml
alert: TrustRecomputeLargeBatch
expr: trust_last_recompute_scene_count > 1000
for: 5m
severity: info
description: "Trust recompute processing unusually large batch ({{ $value }} scenes)"
```

### Grafana Dashboard

A sample Grafana dashboard configuration is available in `docs/grafana/trust-recompute.json` (to be added).

Key panels to include:
1. **Recompute Latency**: Graph showing p50, p95, p99 from `trust_recompute_duration_seconds`
2. **Recompute Rate**: Graph showing `rate(trust_recompute_total[1m])`
3. **Error Rate**: Graph showing `rate(trust_recompute_errors_total[1m])`
4. **Scene Count**: Graph showing `trust_last_recompute_scene_count` over time
5. **Average Variance**: Single stat showing recent `avg_weight_variance` from logs

---

## Configuration

### Environment Variables

See individual sections above for detailed configuration options. Key variables:

- `TRUST_RECOMPUTE_TIMEOUT_SEC` - Timeout for recompute cycles (default: 30)
- `RANK_TRUST_ENABLED` - Feature flag to enable trust-weighted ranking (default: false)

### Performance Tuning

If you observe high latency or timeouts:

1. **Check Database Indexes**: Ensure indexes exist on:
   - `memberships(scene_id)`
   - `alliances(from_scene_id)`
   - `trust_scores(scene_id)`

2. **Review Connection Pool**: Verify database connection pool size is adequate for concurrent recompute operations

3. **Increase Timeout**: Set `TRUST_RECOMPUTE_TIMEOUT_SEC` to a higher value (e.g., 60 or 120 seconds)

4. **Reduce Batch Size**: If processing large batches, consider implementing a dirty scene limit per cycle

### Troubleshooting

**Symptom**: High error rate with timeout messages

**Diagnosis**: Check `trust_recompute_duration_seconds` histogram to identify if timeouts are consistent or sporadic

**Resolution**:
- Sporadic: Likely database query performance; run `EXPLAIN ANALYZE` on slow queries
- Consistent: Increase `TRUST_RECOMPUTE_TIMEOUT_SEC` or optimize batch size

**Symptom**: Stale last recompute timestamp

**Diagnosis**: Check application logs for job startup errors or context cancellation

**Resolution**:
- Verify job is running: Check for "trust recompute job stopping" messages
- Check for context cancellation: Verify parent context is not being cancelled prematurely

---

**Last Updated:** 2026-01-24  
**Next Review:** 2026-02-24 (monthly cadence)
