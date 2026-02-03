# Kubernetes Deployment Manifests

This directory contains Kubernetes deployment manifests for the Subcults API.

## Health Checks

The API deployment includes comprehensive health check configuration:

### Liveness Probe
- **Endpoint**: `GET /health/live`
- **Purpose**: Detect if the service is running
- **Checks**: Lightweight runtime check only (no external dependencies)
- **Timing**: Every 10s after 10s initial delay
- **Action**: Restart container after 3 consecutive failures

### Readiness Probe
- **Endpoint**: `GET /health/ready`
- **Purpose**: Detect if the service can handle traffic
- **Checks**: Database, Redis, LiveKit connectivity (if configured)
- **Timing**: Every 5s after 5s initial delay
- **Action**: Remove from load balancer after 1 failure

### Startup Probe
- **Endpoint**: `GET /health/live`
- **Purpose**: Allow more time during initial container startup
- **Timing**: Every 5s for up to 2 minutes (24 attempts)
- **Action**: Restart container if not healthy within startup period

## Deployment

### Prerequisites

1. Create namespace:
   ```bash
   kubectl create namespace subcults
   ```

2. Create secrets:
   ```bash
   kubectl create secret generic subcults-secrets \
     --namespace=subcults \
     --from-literal=database-url='postgresql://...' \
     --from-literal=jwt-secret='...' \
     --from-literal=redis-url='redis://...' \
     --from-literal=livekit-api-key='...' \
     --from-literal=livekit-api-secret='...'
   ```

3. Create config map:
   ```bash
   kubectl create configmap subcults-config \
     --namespace=subcults \
     --from-literal=livekit-url='wss://...'
   ```

### Apply Manifests

```bash
kubectl apply -f deploy/k8s/api-deployment.yaml
```

### Verify Deployment

Check pod status:
```bash
kubectl get pods -n subcults
```

Check health:
```bash
kubectl exec -it -n subcults deployment/subcults-api -- wget -qO- http://localhost:8080/health/live
kubectl exec -it -n subcults deployment/subcults-api -- wget -qO- http://localhost:8080/health/ready
```

View logs:
```bash
kubectl logs -n subcults deployment/subcults-api --follow
```

## Monitoring

The health check endpoints are exposed at:
- `/health/live` - Liveness check (lightweight)
- `/health/ready` - Readiness check (checks dependencies)

These endpoints return JSON responses:
```json
{
  "status": "up",
  "uptime_s": 3600
}
```

For readiness checks with dependencies:
```json
{
  "status": "up",
  "checks": {
    "db": "ok",
    "redis": "ok",
    "livekit": "ok"
  },
  "uptime_s": 3600
}
```

## Troubleshooting

### Pod keeps restarting
Check liveness probe failures:
```bash
kubectl describe pod -n subcults <pod-name>
```

### Pod not receiving traffic
Check readiness probe status:
```bash
kubectl get pod -n subcults <pod-name> -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}'
```

### Health check timing issues
Adjust probe timings in `api-deployment.yaml`:
- Increase `initialDelaySeconds` for slower startup
- Increase `periodSeconds` for less frequent checks
- Increase `failureThreshold` for more tolerance
