/**
 * Tests for performance monitoring utilities.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
  PERFORMANCE_BUDGETS,
  performanceMark,
  performanceMeasure,
  reportCustomMetric,
} from './performance';

describe('performance monitoring', () => {
  describe('PERFORMANCE_BUDGETS', () => {
    it('should define all Core Web Vitals budgets', () => {
      expect(PERFORMANCE_BUDGETS.FCP).toBe(1000);
      expect(PERFORMANCE_BUDGETS.LCP).toBe(2500);
      expect(PERFORMANCE_BUDGETS.CLS).toBe(0.1);
      expect(PERFORMANCE_BUDGETS.INP).toBe(200);
      expect(PERFORMANCE_BUDGETS.TTFB).toBe(600);
    });

    it('should have budgets within reasonable ranges', () => {
      expect(PERFORMANCE_BUDGETS.FCP).toBeGreaterThan(0);
      expect(PERFORMANCE_BUDGETS.LCP).toBeGreaterThan(PERFORMANCE_BUDGETS.FCP);
      expect(PERFORMANCE_BUDGETS.CLS).toBeLessThan(1);
      expect(PERFORMANCE_BUDGETS.INP).toBeGreaterThan(0);
      expect(PERFORMANCE_BUDGETS.TTFB).toBeGreaterThan(0);
    });
  });

  describe('performanceMark', () => {
    beforeEach(() => {
      vi.clearAllMocks();
    });

    it('should create a performance mark', () => {
      const markSpy = vi.spyOn(performance, 'mark');
      performanceMark('test-mark');
      expect(markSpy).toHaveBeenCalledWith('test-mark');
    });

    it('should handle missing performance API gracefully', () => {
      const originalPerformance = global.performance;
      // @ts-expect-error - intentionally removing for test
      delete global.performance;
      
      expect(() => performanceMark('test')).not.toThrow();
      
      global.performance = originalPerformance;
    });
  });

  describe('performanceMeasure', () => {
    beforeEach(() => {
      performance.clearMarks();
      performance.clearMeasures();
    });

    it('should measure between two marks', () => {
      performanceMark('start');
      performanceMark('end');
      
      const duration = performanceMeasure('test-measure', 'start', 'end');
      
      expect(duration).toBeGreaterThanOrEqual(0);
      expect(typeof duration).toBe('number');
    });

    it('should return null on error', () => {
      // Try to measure without creating marks
      const duration = performanceMeasure('invalid-measure', 'nonexistent-start', 'nonexistent-end');
      
      expect(duration).toBeNull();
    });
  });

  describe('reportCustomMetric', () => {
    let fetchMock: ReturnType<typeof vi.fn>;

    beforeEach(() => {
      fetchMock = vi.fn().mockResolvedValue({ ok: true });
      global.fetch = fetchMock;
    });

    afterEach(() => {
      vi.restoreAllMocks();
    });

    it('should send custom metric to telemetry endpoint', async () => {
      await reportCustomMetric('test-metric', 123.45, { foo: 'bar' });

      expect(fetchMock).toHaveBeenCalledWith(
        '/api/telemetry/custom',
        expect.objectContaining({
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
          },
          keepalive: true,
        })
      );

      const body = JSON.parse(fetchMock.mock.calls[0][1].body);
      expect(body).toMatchObject({
        type: 'custom',
        name: 'test-metric',
        value: 123.45,
        metadata: { foo: 'bar' },
      });
      expect(body.timestamp).toBeGreaterThan(0);
    });

    it('should not throw on fetch error', async () => {
      fetchMock.mockRejectedValue(new Error('Network error'));

      await expect(
        reportCustomMetric('test-metric', 100)
      ).resolves.not.toThrow();
    });
  });
});
