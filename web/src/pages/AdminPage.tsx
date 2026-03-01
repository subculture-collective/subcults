/**
 * AdminPage Component
 * Admin dashboard and controls
 * This is lazy-loaded and protected by admin guard
 * 
 * Features:
 * - Ranking algorithm calibration
 * - User management and search
 * - Moderation controls
 */

import React, { useState } from 'react';
import { RankingCalibrationPanel, type RankingWeights } from '../components/RankingCalibrationPanel';
import { UserSearchBar } from '../components/UserSearchBar';

export const AdminPage: React.FC = () => {
  const [selectedUser, setSelectedUser] = useState<{ id: string; name?: string } | null>(null);

  const handleRankingSubmit = async (weights: RankingWeights) => {
    // TODO: Implement saving ranking weights to backend
    console.log('Saving ranking weights:', weights);
    
    // Simulate API call
    await new Promise((resolve) => setTimeout(resolve, 1000));
    
    return;
  };

  return (
    <div style={{ padding: '2rem', maxWidth: '1200px', margin: '0 auto' }}>
      <h1 style={{ fontSize: '2rem', fontWeight: 700, marginBottom: '2rem' }}>
        Admin Dashboard
      </h1>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '2rem', marginBottom: '2rem' }}>
        {/* Ranking Calibration */}
        <section>
          <h2 style={{ fontSize: '1.25rem', fontWeight: 600, marginBottom: '1rem' }}>
            Ranking Configuration
          </h2>
          <RankingCalibrationPanel onSubmit={handleRankingSubmit} />
        </section>

        {/* User Management */}
        <section>
          <h2 style={{ fontSize: '1.25rem', fontWeight: 600, marginBottom: '1rem' }}>
            User Management
          </h2>
          <UserSearchBar
            placeholder="Search users by name or DID..."
            onSelectUser={setSelectedUser}
            showTrustFilters={true}
          />
          
          {selectedUser && (
            <div style={{
              marginTop: '1rem',
              padding: '1rem',
              backgroundColor: '#f5f5f5',
              borderRadius: '0.5rem',
              border: '1px solid #eee',
            }}>
              <h3 style={{ fontWeight: 600, marginBottom: '0.5rem' }}>
                Selected User
              </h3>
              <p style={{ fontSize: '0.875rem', color: '#666' }}>
                <strong>DID:</strong> {selectedUser.id}
              </p>
              {selectedUser.name && (
                <p style={{ fontSize: '0.875rem', color: '#666' }}>
                  <strong>Name:</strong> {selectedUser.name}
                </p>
              )}
              <button
                onClick={() => setSelectedUser(null)}
                style={{
                  marginTop: '0.75rem',
                  padding: '0.5rem 1rem',
                  backgroundColor: '#f5f5f5',
                  border: '1px solid #ddd',
                  borderRadius: '0.25rem',
                  cursor: 'pointer',
                  fontSize: '0.875rem',
                }}
              >
                Clear Selection
              </button>
            </div>
          )}
        </section>
      </div>

      {/* Moderation Info */}
      <section style={{
        padding: '1.5rem',
        backgroundColor: '#f9f9f9',
        border: '1px solid #eee',
        borderRadius: '0.5rem',
      }}>
        <h2 style={{ fontSize: '1.25rem', fontWeight: 600, marginBottom: '1rem' }}>
          Moderation & Controls
        </h2>
        <p style={{ color: '#666', marginBottom: '1rem' }}>
          Advanced moderation tools and controls will be available here.
        </p>
        <ul style={{ color: '#666', fontSize: '0.875rem', lineHeight: '1.6' }}>
          <li>✓ Scene/event moderation (pause, suspend, block)</li>
          <li>✓ User account management (warn, suspend, ban)</li>
          <li>✓ Content violation reporting</li>
          <li>✓ Audit logs and compliance tracking</li>
          <li>✓ Feature flag management</li>
        </ul>
      </section>
    </div>
  );
};
