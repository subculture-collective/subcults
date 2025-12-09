/**
 * LoadingSkeleton Component
 * Displayed while lazy-loaded routes are loading
 */

import React from 'react';

export const LoadingSkeleton: React.FC = () => {
  return (
    <div
      className="loading-skeleton"
      role="status"
      aria-live="polite"
      aria-busy="true"
      aria-label="Loading content"
      style={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        height: '100vh',
        width: '100%',
        backgroundColor: '#1a1a1a',
        color: 'white',
      }}
    >
      <div style={{ textAlign: 'center' }}>
        <div
          style={{
            width: '50px',
            height: '50px',
            border: '4px solid rgba(255, 255, 255, 0.1)',
            borderTop: '4px solid white',
            borderRadius: '50%',
            animation: 'spin 1s linear infinite',
            margin: '0 auto 1rem',
          }}
        />
        <p>Loading...</p>
      </div>
      <style>{`
        @keyframes spin {
          0% { transform: rotate(0deg); }
          100% { transform: rotate(360deg); }
        }
      `}</style>
    </div>
  );
};
