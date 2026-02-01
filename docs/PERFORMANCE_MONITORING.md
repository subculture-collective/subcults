# Performance Monitoring Guide

## Overview

Subcults implements comprehensive performance monitoring and budgeting to ensure a fast, responsive user experience. This guide covers the performance monitoring infrastructure, budgets, and workflows.

## Performance Budgets

### Core Web Vitals

The following Core Web Vitals budgets are enforced:

| Metric | Budget | Description |
|--------|--------|-------------|
| **FCP** | <1.0s | First Contentful Paint - when first content appears |
| **LCP** | <2.5s | Largest Contentful Paint - when main content is visible |
| **CLS** | <0.1 | Cumulative Layout Shift - visual stability (unitless) |
| **INP** | <200ms | Interaction to Next Paint - responsiveness |
| **TTFB** | <600ms | Time to First Byte - server response time |

### Bundle Size Budgets

| Bundle | Budget (gzipped) | Description |
|--------|------------------|-------------|
| Main JS | 200 KB | Main application bundle |
| Main CSS | 50 KB | Main stylesheet bundle |
| Vendor JS | 300 KB | Dependencies/vendor bundle |
| Total JS | 500 KB | All JavaScript combined |
| MapLibre | 450 KB | Map rendering library |

### Lighthouse Scores

| Category | Minimum Score |
|----------|---------------|
| Performance | 90/100 |
| Accessibility | 90/100 |
| Best Practices | 90/100 |
| SEO | 90/100 |

### Regression Thresholds

- **Bundle Size Increase**: Maximum 10% allowed
- **Performance Score Decrease**: Maximum 5% allowed

Builds that exceed these thresholds will fail in CI.

## Architecture

### Frontend Monitoring

**Location**: `web/src/lib/performance.ts`

The frontend uses the `web-vitals` library to collect Core Web Vitals metrics:

```typescript
import { initializePerformanceMonitoring } from './lib/performance';

// In main.tsx
initializePerformanceMonitoring();
```

Metrics are automatically:
1. Collected via Performance Observer API
2. Logged to console in development mode
3. Sent to backend telemetry endpoint (`/api/telemetry/web-vitals`)

### React Hooks

**Location**: `web/src/hooks/usePerformance.ts`

Access performance data in React components:

```typescript
import { usePerformance } from '../hooks/usePerformance';

function MyComponent() {
  const { metrics, hasViolations, violations } = usePerformance();
  
  if (hasViolations) {
    console.warn('Performance violations:', violations);
  }
  
  return <div>LCP: {metrics.LCP?.value.toFixed(2)}ms</div>;
}
```

### Performance Dashboard (Development Only)

**Location**: `web/src/components/PerformanceDashboard.tsx`

A floating dashboard that shows real-time Web Vitals in development:

```typescript
import { PerformanceDashboard } from './components/PerformanceDashboard';

function App() {
  return (
    <>
      <YourApp />
      <PerformanceDashboard />
    </>
  );
}
```

## Lighthouse CI

### Configuration

**Location**: `lighthouserc.json`

Lighthouse CI runs automated performance audits on every PR and push to main/develop branches.

Key configuration:
- **Runs**: 3 audits per build (median score used)
- **Preset**: Desktop performance
- **Categories**: Performance, Accessibility, Best Practices, SEO
- **Storage**: Temporary public storage (results accessible for 7 days)

### GitHub Actions Workflow

**Location**: `.github/workflows/performance.yml`

The workflow runs on:
- Pull requests that modify `web/**` or `lighthouserc.json`
- Pushes to `main` or `develop` branches

Steps:
1. Build the frontend
2. Run Lighthouse CI with 3 audits
3. Assert performance budgets
4. Upload reports as artifacts
5. Check bundle sizes
6. Fail build if budgets exceeded

### Viewing Reports

After a CI run:

1. Go to the GitHub Actions run
2. Click on the "Lighthouse CI" job
3. Download the `lighthouse-reports` artifact
4. Open `.lighthouseci/lhr-*.html` files in a browser

Or use the Lighthouse CI dashboard link in the console output.

## Bundle Analysis

### Rollup Visualizer

**Location**: `web/vite.config.ts`

Bundle analysis is automatically generated during builds:

```bash
cd web
npm run build
```

This creates `dist/stats.html` which visualizes:
- Bundle composition
- Module sizes (raw, gzipped, brotli)
- Dependency tree
- Optimization opportunities

Open `dist/stats.html` in a browser to explore the bundle.

### CI Bundle Checks

The `performance-budgets` job in CI automatically:
1. Builds the frontend
2. Calculates gzipped sizes for all bundles
3. Compares against budgets
4. Fails if any budget is exceeded

## Backend Telemetry

### Endpoint

**Location**: `internal/api/telemetry_handlers.go` (to be created)

Frontend metrics are sent to:
```
POST /api/telemetry/web-vitals
```

Expected payload:
```json
{
  "name": "LCP",
  "value": 1234.56,
  "rating": "good",
  "delta": 100.00,
  "id": "v3-1234567890",
  "navigationType": "navigate",
  "timestamp": 1704067200000,
  "budget": 2500,
  "exceedsBudget": false
}
```

### Storage

Metrics can be:
1. Aggregated in-memory for Prometheus export
2. Stored in database for historical analysis
3. Forwarded to external monitoring (e.g., Datadog, New Relic)

### Privacy

Telemetry follows Subcults privacy principles:
- No PII in metrics
- Anonymous user identifiers only
- No tracking across sessions
- Aggregate statistics only

## Development Workflow

### Local Development

1. **Install dependencies**:
   ```bash
   cd web
   npm install
   ```

2. **Run dev server**:
   ```bash
   npm run dev
   ```

3. **Monitor metrics**: Open browser console to see Web Vitals logs

4. **View dashboard**: The performance dashboard appears in bottom-right corner

### Before Submitting PR

1. **Build and check bundle size**:
   ```bash
   cd web
   npm run build
   open dist/stats.html
   ```

2. **Run Lighthouse locally** (optional):
   ```bash
   # Build first
   cd web
   npm run build
   
   # Run Lighthouse CI
   cd ..
   npx @lhci/cli autorun
   ```

3. **Review violations**: Fix any budget violations before pushing

### Optimizing Performance

If budgets are exceeded:

1. **Analyze bundle**:
   - Open `dist/stats.html`
   - Identify large dependencies
   - Look for duplicate modules

2. **Common fixes**:
   - Lazy-load routes: `React.lazy(() => import('./Route'))`
   - Code-split heavy dependencies
   - Use dynamic imports for conditional features
   - Optimize images (WebP, lazy loading)
   - Tree-shake unused code
   - Minimize CSS (Tailwind purge)

3. **Re-test**:
   ```bash
   npm run build
   npx @lhci/cli autorun
   ```

## CI/CD Integration

### Pull Request Workflow

1. Developer pushes changes to PR
2. `performance.yml` workflow triggers
3. Frontend is built
4. Lighthouse CI runs 3 audits
5. Bundle sizes are checked
6. Results posted as PR comment (if configured)
7. Build fails if budgets exceeded

### Main Branch Workflow

Same as PR, but results are stored for trend analysis.

## Monitoring in Production

### Metrics Collection

Production metrics are sent to `/api/telemetry/web-vitals` and:
- Aggregated by Prometheus
- Visualized in Grafana dashboards
- Alerted via Prometheus Alertmanager

### Recommended Queries

**Prometheus queries** (to be configured):

```promql
# P95 LCP over last hour
histogram_quantile(0.95, rate(web_vitals_lcp_bucket[1h]))

# Percentage of sessions exceeding FCP budget
sum(rate(web_vitals_fcp_bucket{le="1000"}[5m])) 
  / sum(rate(web_vitals_fcp_count[5m]))

# Budget violation rate
rate(web_vitals_budget_violations_total[5m])
```

### Alerts

Recommended alerts:
- P95 LCP > 3s for 10 minutes
- P95 FCP > 1.5s for 10 minutes
- CLS > 0.2 for any session
- Budget violation rate > 10% of sessions

## Troubleshooting

### High LCP

Causes:
- Large images not optimized
- Render-blocking resources
- Slow server response (TTFB)
- JavaScript execution blocking render

Fixes:
- Optimize images (WebP, lazy loading, responsive sizes)
- Preload critical resources
- Defer non-critical scripts
- Optimize backend API response times

### High CLS

Causes:
- Images without dimensions
- Dynamically injected content
- Web fonts causing layout shifts

Fixes:
- Set explicit width/height on images
- Reserve space for dynamic content
- Use font-display: swap with size-adjust

### Large Bundle Size

Causes:
- Heavy dependencies
- Duplicate modules
- Unused code not tree-shaken

Fixes:
- Analyze bundle with visualizer
- Replace heavy dependencies (e.g., moment.js → date-fns)
- Enable tree-shaking
- Code-split routes and features

### CI Failures

If CI fails due to performance budgets:

1. **Review the failure**:
   - Check GitHub Actions logs
   - Download Lighthouse reports artifact
   - Identify which budget failed

2. **Compare to baseline**:
   - Check previous successful builds
   - Identify what changed (git diff)

3. **Fix or adjust**:
   - Option A: Optimize to meet budget
   - Option B: Adjust budget if necessary (requires justification)

## Configuration Files

| File | Purpose |
|------|---------|
| `lighthouserc.json` | Lighthouse CI configuration |
| `web/src/config/performance-budgets.ts` | Budget definitions |
| `web/src/lib/performance.ts` | Web Vitals monitoring |
| `.github/workflows/performance.yml` | CI workflow |
| `web/vite.config.ts` | Bundle analyzer plugin |

## Resources

- [Web Vitals](https://web.dev/vitals/)
- [Lighthouse CI](https://github.com/GoogleChrome/lighthouse-ci)
- [Performance Budgets 101](https://web.dev/performance-budgets-101/)
- [Core Web Vitals Workflow](https://web.dev/vitals-measurement-getting-started/)
- [Bundle Analysis](https://github.com/btd/rollup-plugin-visualizer)

## Future Enhancements

Planned improvements:
- [ ] Real-time performance dashboard in admin panel
- [ ] Historical trend analysis and reporting
- [ ] Automated performance regression detection
- [ ] A/B testing framework for performance experiments
- [ ] Resource timing API integration
- [ ] Long task monitoring
- [ ] User-centric performance scoring
