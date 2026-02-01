/**
 * Performance monitoring dashboard component.
 * 
 * Displays Core Web Vitals metrics in development mode.
 * Only renders in development to avoid adding overhead in production.
 */

import { usePerformance } from '../hooks/usePerformance';
import { PERFORMANCE_BUDGETS } from '../lib/performance';

export function PerformanceDashboard() {
  const { metrics, hasViolations, violations } = usePerformance();

  // Only show in development
  if (import.meta.env.PROD) {
    return null;
  }

  return (
    <div
      style={{
        position: 'fixed',
        bottom: '10px',
        right: '10px',
        backgroundColor: 'rgba(0, 0, 0, 0.8)',
        color: 'white',
        padding: '12px',
        borderRadius: '8px',
        fontSize: '11px',
        fontFamily: 'monospace',
        zIndex: 9999,
        maxWidth: '300px',
        boxShadow: '0 4px 6px rgba(0, 0, 0, 0.3)',
      }}
    >
      <div style={{ fontWeight: 'bold', marginBottom: '8px', fontSize: '12px' }}>
        ⚡ Web Vitals
      </div>
      
      {Object.entries(metrics).map(([name, metric]) => {
        if (!metric) return null;
        
        const budget = PERFORMANCE_BUDGETS[name as keyof typeof PERFORMANCE_BUDGETS];
        const exceedsBudget = budget !== undefined && metric.value > budget;
        const emoji = exceedsBudget ? '🔴' : '🟢';
        
        return (
          <div
            key={name}
            style={{
              marginBottom: '4px',
              color: exceedsBudget ? '#ff6b6b' : '#51cf66',
            }}
          >
            {emoji} {name}: {metric.value.toFixed(2)}
            {name !== 'CLS' && 'ms'} ({metric.rating})
          </div>
        );
      })}
      
      {hasViolations && (
        <div
          style={{
            marginTop: '8px',
            padding: '6px',
            backgroundColor: 'rgba(255, 107, 107, 0.2)',
            borderRadius: '4px',
            fontSize: '10px',
          }}
        >
          <div style={{ fontWeight: 'bold', marginBottom: '4px' }}>
            ⚠️ Budget Violations:
          </div>
          {violations.map((violation, idx) => (
            <div key={idx}>• {violation}</div>
          ))}
        </div>
      )}
    </div>
  );
}
