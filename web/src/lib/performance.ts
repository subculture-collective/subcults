/**
 * Performance monitoring utilities for Web Vitals and custom metrics.
 * 
 * This module collects Core Web Vitals (FCP, LCP, CLS, INP, TTFB) and reports them
 * to the backend telemetry endpoint for aggregation and monitoring.
 * 
 * Performance budgets:
 * - FCP: <1.0s
 * - LCP: <2.5s
 * - CLS: <0.1
 * - INP: <200ms
 * - TTFB: <600ms
 */

import { onCLS, onFCP, onINP, onLCP, onTTFB, type Metric } from 'web-vitals';

// Telemetry endpoint configuration
const TELEMETRY_ENDPOINT = '/api/telemetry/web-vitals';

// Performance budget thresholds
export const PERFORMANCE_BUDGETS = {
  FCP: 1000,    // First Contentful Paint: 1.0s
  LCP: 2500,    // Largest Contentful Paint: 2.5s
  CLS: 0.1,     // Cumulative Layout Shift: 0.1
  INP: 200,     // Interaction to Next Paint: 200ms
  TTFB: 600,    // Time to First Byte: 600ms
} as const;

export interface WebVitalsPayload {
  name: string;
  value: number;
  rating: 'good' | 'needs-improvement' | 'poor';
  delta: number;
  id: string;
  navigationType: string;
  timestamp: number;
  budget: number;
  exceedsBudget: boolean;
}

/**
 * Checks if a metric exceeds its performance budget.
 */
function exceedsBudget(metric: Metric): boolean {
  const budget = PERFORMANCE_BUDGETS[metric.name as keyof typeof PERFORMANCE_BUDGETS];
  if (budget === undefined) {
    return false;
  }
  return metric.value > budget;
}

/**
 * Sends a metric to the backend telemetry endpoint.
 */
async function sendToTelemetry(payload: WebVitalsPayload): Promise<void> {
  try {
    // Use sendBeacon API if available for reliability during page unload
    if (navigator.sendBeacon) {
      const blob = new Blob([JSON.stringify(payload)], { type: 'application/json' });
      navigator.sendBeacon(TELEMETRY_ENDPOINT, blob);
    } else {
      // Fallback to fetch with keepalive
      await fetch(TELEMETRY_ENDPOINT, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(payload),
        keepalive: true,
      });
    }
  } catch (error) {
    // Silently fail - don't impact user experience for telemetry errors
    if (import.meta.env.DEV) {
      console.warn('[Performance] Failed to send metric to telemetry:', error);
    }
  }
}

/**
 * Handles a Web Vitals metric by logging it and sending to telemetry.
 */
function handleMetric(metric: Metric): void {
  const payload: WebVitalsPayload = {
    name: metric.name,
    value: metric.value,
    rating: metric.rating,
    delta: metric.delta,
    id: metric.id,
    navigationType: metric.navigationType,
    timestamp: Date.now(),
    budget: PERFORMANCE_BUDGETS[metric.name as keyof typeof PERFORMANCE_BUDGETS] || 0,
    exceedsBudget: exceedsBudget(metric),
  };

  // Log to console in development
  if (import.meta.env.DEV) {
    const emoji = payload.exceedsBudget ? '⚠️' : '✅';
    console.log(
      `[Performance] ${emoji} ${metric.name}: ${metric.value.toFixed(2)} (${metric.rating})`,
      payload
    );
  }

  // Send to backend telemetry
  sendToTelemetry(payload);
}

/**
 * Initializes Web Vitals monitoring.
 * Should be called once during app initialization.
 */
export function initializePerformanceMonitoring(): void {
  // Register all Core Web Vitals observers
  onCLS(handleMetric);
  onFCP(handleMetric);
  onINP(handleMetric);
  onLCP(handleMetric);
  onTTFB(handleMetric);

  if (import.meta.env.DEV) {
    console.log('[Performance] Monitoring initialized with budgets:', PERFORMANCE_BUDGETS);
  }
}

/**
 * Custom performance mark for measuring specific operations.
 */
export function performanceMark(name: string): void {
  if ('performance' in window && performance.mark) {
    performance.mark(name);
  }
}

/**
 * Custom performance measure between two marks.
 */
export function performanceMeasure(
  name: string,
  startMark: string,
  endMark?: string
): number | null {
  if ('performance' in window && performance.measure) {
    try {
      const measure = performance.measure(name, startMark, endMark);
      return measure.duration;
    } catch (error) {
      if (import.meta.env.DEV) {
        console.warn(`[Performance] Failed to measure ${name}:`, error);
      }
      return null;
    }
  }
  return null;
}

/**
 * Reports custom metric to telemetry.
 */
export async function reportCustomMetric(
  name: string,
  value: number,
  metadata?: Record<string, unknown>
): Promise<void> {
  try {
    const payload = {
      type: 'custom',
      name,
      value,
      timestamp: Date.now(),
      metadata,
    };

    await fetch('/api/telemetry/custom', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(payload),
      keepalive: true,
    });
  } catch (error) {
    if (import.meta.env.DEV) {
      console.warn('[Performance] Failed to report custom metric:', error);
    }
  }
}
