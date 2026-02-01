/**
 * React hook for accessing Web Vitals performance data.
 * 
 * This hook provides real-time access to Core Web Vitals metrics
 * and allows components to react to performance changes.
 */

import { useEffect, useState } from 'react';
import { onCLS, onFCP, onINP, onLCP, onTTFB, type Metric } from 'web-vitals';
import { PERFORMANCE_BUDGETS } from '../lib/performance';

export interface PerformanceMetrics {
  FCP: Metric | null;
  LCP: Metric | null;
  CLS: Metric | null;
  INP: Metric | null;
  TTFB: Metric | null;
}

export interface PerformanceStatus {
  metrics: PerformanceMetrics;
  isLoading: boolean;
  hasViolations: boolean;
  violations: string[];
}

/**
 * Hook to monitor Web Vitals performance metrics.
 * 
 * @returns Current performance metrics and status
 * 
 * @example
 * ```tsx
 * function PerformanceMonitor() {
 *   const { metrics, hasViolations, violations } = usePerformance();
 *   
 *   if (hasViolations) {
 *     console.warn('Performance violations:', violations);
 *   }
 *   
 *   return <div>FCP: {metrics.FCP?.value.toFixed(2)}ms</div>;
 * }
 * ```
 */
export function usePerformance(): PerformanceStatus {
  const [metrics, setMetrics] = useState<PerformanceMetrics>({
    FCP: null,
    LCP: null,
    CLS: null,
    INP: null,
    TTFB: null,
  });

  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    // Track if component is mounted to prevent state updates after unmount
    let mounted = true;

    // Register metric observers
    const updateMetric = (metric: Metric) => {
      if (mounted) {
        setMetrics((prev) => ({
          ...prev,
          [metric.name]: metric,
        }));
      }
    };

    onCLS(updateMetric);
    onFCP(updateMetric);
    onINP(updateMetric);
    onLCP(updateMetric);
    onTTFB(updateMetric);

    // Mark as loaded after initial metrics are collected
    const loadTimer = setTimeout(() => {
      if (mounted) {
        setIsLoading(false);
      }
    }, 100);

    return () => {
      mounted = false;
      clearTimeout(loadTimer);
    };
  }, []);

  // Calculate violations
  const violations: string[] = [];
  let hasViolations = false;

  Object.entries(metrics).forEach(([name, metric]) => {
    if (metric) {
      const budget = PERFORMANCE_BUDGETS[name as keyof typeof PERFORMANCE_BUDGETS];
      if (budget !== undefined && metric.value > budget) {
        violations.push(`${name}: ${metric.value.toFixed(2)} > ${budget}`);
        hasViolations = true;
      }
    }
  });

  return {
    metrics,
    isLoading,
    hasViolations,
    violations,
  };
}

/**
 * Hook to track custom performance marks and measures.
 * 
 * @param markName - Name of the performance mark
 * @returns Functions to create marks and measures
 * 
 * @example
 * ```tsx
 * function DataLoader() {
 *   const { mark, measure } = usePerformanceMark('data-load');
 *   
 *   useEffect(() => {
 *     mark('start');
 *     fetchData().then(() => {
 *       mark('end');
 *       const duration = measure('start', 'end');
 *       console.log(`Data loaded in ${duration}ms`);
 *     });
 *   }, []);
 * }
 * ```
 */
export function usePerformanceMark(markName: string) {
  const mark = (label: string) => {
    if ('performance' in window && performance.mark) {
      performance.mark(`${markName}-${label}`);
    }
  };

  const measure = (startLabel: string, endLabel?: string): number | null => {
    if ('performance' in window && performance.measure) {
      try {
        const measureName = `${markName}-measure`;
        const startMark = `${markName}-${startLabel}`;
        const endMark = endLabel ? `${markName}-${endLabel}` : undefined;
        
        const entry = performance.measure(measureName, startMark, endMark);
        return entry.duration;
      } catch (error) {
        if (import.meta.env.DEV) {
          console.warn(`Failed to measure ${markName}:`, error);
        }
        return null;
      }
    }
    return null;
  };

  return { mark, measure };
}
