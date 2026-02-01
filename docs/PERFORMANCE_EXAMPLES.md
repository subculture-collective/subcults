# Performance Monitoring Usage Examples

This document provides practical examples of using the performance monitoring infrastructure.

## Basic Setup

The performance monitoring is automatically initialized in `main.tsx`:

```typescript
import { initializePerformanceMonitoring } from './lib/performance';

// Automatically collects and reports Core Web Vitals
initializePerformanceMonitoring();
```

## Using Web Vitals in Components

### Display Performance Metrics

```typescript
import { usePerformance } from '../hooks/usePerformance';

function PerformanceIndicator() {
  const { metrics, hasViolations } = usePerformance();
  
  return (
    <div>
      {metrics.LCP && (
        <span className={metrics.LCP.rating === 'good' ? 'text-green-500' : 'text-red-500'}>
          LCP: {metrics.LCP.value.toFixed(2)}ms
        </span>
      )}
      {hasViolations && (
        <div className="text-yellow-500">⚠️ Performance issues detected</div>
      )}
    </div>
  );
}
```

### Monitor Specific Operations

```typescript
import { usePerformanceMark } from '../hooks/usePerformance';
import { useEffect } from 'react';

function DataLoader() {
  const { mark, measure } = usePerformanceMark('data-fetch');
  
  useEffect(() => {
    async function loadData() {
      mark('start');
      
      const response = await fetch('/api/data');
      const data = await response.json();
      
      mark('end');
      const duration = measure('start', 'end');
      
      console.log(`Data loaded in ${duration}ms`);
    }
    
    loadData();
  }, [mark, measure]);
  
  return <div>Loading...</div>;
}
```

## Custom Performance Metrics

### Report Custom Timing

```typescript
import { reportCustomMetric } from '../lib/performance';

async function processLargeDataset(data: unknown[]) {
  const startTime = performance.now();
  
  // Process data...
  const result = await heavyComputation(data);
  
  const duration = performance.now() - startTime;
  
  // Report to backend
  await reportCustomMetric('dataset-processing', duration, {
    recordCount: data.length,
    cacheHit: false,
  });
  
  return result;
}
```

### Track Feature Usage

```typescript
import { reportCustomMetric } from '../lib/performance';

function MapComponent() {
  const handleZoom = async (level: number) => {
    // Track zoom level changes
    await reportCustomMetric('map-zoom', level, {
      source: 'user-interaction',
    });
  };
  
  return <Map onZoomChange={handleZoom} />;
}
```

## Performance Marks and Measures

### Measure Route Changes

```typescript
import { performanceMark, performanceMeasure } from '../lib/performance';
import { useEffect } from 'react';
import { useLocation } from 'react-router-dom';

function RouteTracker() {
  const location = useLocation();
  
  useEffect(() => {
    performanceMark('route-start');
    
    return () => {
      performanceMark('route-end');
      const duration = performanceMeasure('route-transition', 'route-start', 'route-end');
      console.log(`Route transition took ${duration}ms`);
    };
  }, [location.pathname]);
  
  return null;
}
```

## Development Mode Dashboard

Enable the performance dashboard during development:

```typescript
import { PerformanceDashboard } from './components/PerformanceDashboard';

function App() {
  return (
    <>
      <Router>
        <Routes>{/* Your routes */}</Routes>
      </Router>
      
      {/* Shows metrics in bottom-right corner (dev mode only) */}
      <PerformanceDashboard />
    </>
  );
}
```

## CI/CD Integration

### Local Lighthouse Audit

Run Lighthouse CI locally before pushing:

```bash
# Build the frontend
cd web
npm run build

# Run Lighthouse CI
cd ..
npx @lhci/cli autorun

# View results
open .lighthouseci/lhr-*.html
```

### Bundle Analysis

Generate and view bundle analysis:

```bash
cd web
npm run build
open dist/stats.html
```

## Optimizing Performance

### Code Splitting

```typescript
// Before: Bundle includes all pages
import HomePage from './pages/HomePage';
import ProfilePage from './pages/ProfilePage';

// After: Lazy load pages
const HomePage = lazy(() => import('./pages/HomePage'));
const ProfilePage = lazy(() => import('./pages/ProfilePage'));

function App() {
  return (
    <Suspense fallback={<Loading />}>
      <Routes>
        <Route path="/" element={<HomePage />} />
        <Route path="/profile" element={<ProfilePage />} />
      </Routes>
    </Suspense>
  );
}
```

### Image Optimization

```typescript
// Use native lazy loading
<img
  src="/images/hero.jpg"
  alt="Hero"
  loading="lazy"
  width={800}
  height={600}
/>

// Or with React component
import { lazy } from 'react';

function Hero() {
  return (
    <picture>
      <source srcSet="/images/hero.webp" type="image/webp" />
      <img
        src="/images/hero.jpg"
        alt="Hero"
        loading="lazy"
        width={800}
        height={600}
      />
    </picture>
  );
}
```

### Debounce Heavy Operations

```typescript
import { useEffect, useState } from 'react';

function SearchComponent() {
  const [query, setQuery] = useState('');
  const [debouncedQuery, setDebouncedQuery] = useState('');
  
  // Debounce search to reduce API calls
  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedQuery(query);
    }, 300);
    
    return () => clearTimeout(timer);
  }, [query]);
  
  // Only search when debounced value changes
  useEffect(() => {
    if (debouncedQuery) {
      performSearch(debouncedQuery);
    }
  }, [debouncedQuery]);
  
  return (
    <input
      type="text"
      value={query}
      onChange={(e) => setQuery(e.target.value)}
    />
  );
}
```

## Budget Violations

When performance budgets are violated:

1. **Check the metric**: Identify which metric exceeded the budget
2. **Analyze the cause**: Use Lighthouse reports and bundle analysis
3. **Optimize**: Apply appropriate optimizations
4. **Re-test**: Run Lighthouse CI locally
5. **Document**: If budget adjustment is needed, update configuration and document reasoning

## Best Practices

1. **Monitor in production**: Don't disable in production; use for real user data
2. **Set realistic budgets**: Based on your users' devices and networks
3. **Test on slow networks**: Use Chrome DevTools network throttling
4. **Optimize images**: Use WebP, lazy loading, responsive sizes
5. **Code-split routes**: Load pages on-demand
6. **Minimize JavaScript**: Tree-shake unused code
7. **Defer non-critical**: Load analytics, chat widgets after main content
8. **Cache aggressively**: Use service workers and HTTP caching

## Resources

- [Web Vitals Documentation](https://web.dev/vitals/)
- [Lighthouse CI Documentation](https://github.com/GoogleChrome/lighthouse-ci)
- [React Performance Optimization](https://react.dev/learn/render-and-commit)
- [Vite Performance Guide](https://vitejs.dev/guide/performance.html)
