/**
 * Performance budgets for Subcults frontend.
 * 
 * These budgets are enforced by:
 * 1. Lighthouse CI in GitHub Actions
 * 2. Web Vitals monitoring in production
 * 3. Bundle size tracking via rollup-plugin-visualizer
 * 
 * @see https://web.dev/performance-budgets-101/
 */

export const performanceBudgets = {
  /**
   * Core Web Vitals budgets (milliseconds)
   */
  webVitals: {
    /** First Contentful Paint - when first content appears */
    FCP: 1000,
    
    /** Largest Contentful Paint - when main content is visible */
    LCP: 2500,
    
    /** Cumulative Layout Shift - visual stability (unitless) */
    CLS: 0.1,
    
    /** Interaction to Next Paint - responsiveness */
    INP: 200,
    
    /** Time to First Byte - server response time */
    TTFB: 600,
  },

  /**
   * Bundle size budgets (kilobytes, gzipped)
   */
  bundleSize: {
    /** Main JavaScript bundle */
    mainJS: 200,
    
    /** Main CSS bundle */
    mainCSS: 50,
    
    /** Vendor/dependencies bundle */
    vendorJS: 300,
    
    /** Total JavaScript (all bundles combined) */
    totalJS: 500,
    
    /** MapLibre GL library (loaded separately) */
    mapLibreJS: 450,
  },

  /**
   * Regression thresholds
   */
  regressionThresholds: {
    /** Maximum allowed bundle size increase (percentage) */
    bundleSizeIncrease: 10,
    
    /** Maximum allowed performance score decrease (percentage) */
    performanceScoreDecrease: 5,
  },

  /**
   * Lighthouse performance score targets
   */
  lighthouse: {
    /** Minimum acceptable performance score (0-100) */
    performance: 90,
    
    /** Minimum acceptable accessibility score (0-100) */
    accessibility: 90,
    
    /** Minimum acceptable best practices score (0-100) */
    bestPractices: 90,
    
    /** Minimum acceptable SEO score (0-100) */
    seo: 90,
  },
} as const;

export type PerformanceBudgets = typeof performanceBudgets;
