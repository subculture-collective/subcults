/**
 * LoadingSkeleton Component
 * Displayed while lazy-loaded routes are loading
 */

import React from 'react';

export const LoadingSkeleton: React.FC = () => {
  return (
    <div
      className="flex items-center justify-center h-screen w-full bg-brand-underground text-white"
      role="status"
      aria-live="polite"
      aria-busy="true"
      aria-label="Loading content"
    >
      <div className="text-center">
        <div className="w-[50px] h-[50px] border-4 border-white/10 border-t-white rounded-full animate-spin mx-auto mb-4" />
        <p>Loading...</p>
      </div>
    </div>
  );
};
