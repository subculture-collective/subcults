/**
 * LoginPage Component
 * User login form
 */

import React from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import { authStore } from '../stores/authStore';

export const LoginPage: React.FC = () => {
  const navigate = useNavigate();
  const location = useLocation();

  // Get the page they tried to visit, or default to home
  const from =
    (location.state as { from?: { pathname: string } } | null)?.from?.pathname ||
    '/';

  const handleLogin = (role: 'user' | 'admin') => {
    // Placeholder login - will be replaced with real auth
    authStore.setUser(
      {
        did: 'did:example:test-user',
        role,
      },
      'placeholder-access-token' // Placeholder token
    );
    navigate(from, { replace: true });
  };

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        minHeight: '100vh',
        padding: '2rem',
      }}
    >
      <h1>Login</h1>
      <p style={{ marginBottom: '2rem' }}>
        Placeholder login - click to simulate authentication
      </p>
      <div style={{ display: 'flex', gap: '1rem' }}>
        <button
          onClick={() => handleLogin('user')}
          style={{
            padding: '0.75rem 1.5rem',
            fontSize: '1rem',
            cursor: 'pointer',
          }}
        >
          Login as User
        </button>
        <button
          onClick={() => handleLogin('admin')}
          style={{
            padding: '0.75rem 1.5rem',
            fontSize: '1rem',
            cursor: 'pointer',
          }}
        >
          Login as Admin
        </button>
      </div>
    </div>
  );
};
