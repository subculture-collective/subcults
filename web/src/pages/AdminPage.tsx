/**
 * AdminPage Component
 * Admin dashboard and controls
 * This is lazy-loaded and protected by admin guard
 */

import React from 'react';

export const AdminPage: React.FC = () => {
  return (
    <div style={{ padding: '2rem' }}>
      <h1>Admin Dashboard</h1>
      <p>Admin controls and monitoring tools will be displayed here.</p>
      <p>This page is only accessible to users with admin role.</p>
      <p>This page is lazy-loaded for performance.</p>
    </div>
  );
};
