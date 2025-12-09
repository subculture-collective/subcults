/**
 * Route tests
 * Tests for routing behavior, guards, and navigation
 */

import { describe, it, expect, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Routes, Route, useLocation } from 'react-router-dom';
import { RequireAuth } from '../guards/RequireAuth';
import { RequireAdmin } from '../guards/RequireAdmin';
import { authStore } from '../stores/authStore';

// Mock pages for testing
const MockPublicPage = () => <div>Public Page</div>;
const MockProtectedPage = () => <div>Protected Page</div>;
const MockAdminPage = () => <div>Admin Page</div>;
interface LocationState {
  from?: { pathname: string };
}

const MockLoginPage = () => {
  const location = useLocation();
  const state = location.state as LocationState | null;
  const from = state?.from?.pathname;
  return (
    <div>
      Login Page
      {from && <span data-testid="return-url">{from}</span>}
    </div>
  );
};

describe('Route Guards', () => {
  beforeEach(() => {
    // Reset auth state before each test
    authStore.logout();
  });

  describe('RequireAuth', () => {
    it('redirects to login when not authenticated', async () => {
      render(
        <MemoryRouter initialEntries={['/protected']}>
          <Routes>
            <Route path="/account/login" element={<MockLoginPage />} />
            <Route
              path="/protected"
              element={
                <RequireAuth>
                  <MockProtectedPage />
                </RequireAuth>
              }
            />
          </Routes>
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByText('Login Page')).toBeInTheDocument();
      });
    });

    it('preserves return URL when redirecting to login', async () => {
      render(
        <MemoryRouter initialEntries={['/protected']}>
          <Routes>
            <Route path="/account/login" element={<MockLoginPage />} />
            <Route
              path="/protected"
              element={
                <RequireAuth>
                  <MockProtectedPage />
                </RequireAuth>
              }
            />
          </Routes>
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByText('Login Page')).toBeInTheDocument();
        expect(screen.getByTestId('return-url')).toHaveTextContent('/protected');
      });
    });

    it('renders protected content when authenticated', async () => {
      authStore.setUser({ did: 'did:test:user', role: 'user' });

      render(
        <MemoryRouter initialEntries={['/protected']}>
          <Routes>
            <Route path="/account/login" element={<MockLoginPage />} />
            <Route
              path="/protected"
              element={
                <RequireAuth>
                  <MockProtectedPage />
                </RequireAuth>
              }
            />
          </Routes>
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByText('Protected Page')).toBeInTheDocument();
      });
    });
  });

  describe('RequireAdmin', () => {
    it('redirects to login when not authenticated', async () => {
      render(
        <MemoryRouter initialEntries={['/admin']}>
          <Routes>
            <Route path="/account/login" element={<MockLoginPage />} />
            <Route path="/" element={<MockPublicPage />} />
            <Route
              path="/admin"
              element={
                <RequireAdmin>
                  <MockAdminPage />
                </RequireAdmin>
              }
            />
          </Routes>
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByText('Login Page')).toBeInTheDocument();
      });
    });

    it('redirects to home when authenticated but not admin', async () => {
      authStore.setUser({ did: 'did:test:user', role: 'user' });

      render(
        <MemoryRouter initialEntries={['/admin']}>
          <Routes>
            <Route path="/account/login" element={<MockLoginPage />} />
            <Route path="/" element={<MockPublicPage />} />
            <Route
              path="/admin"
              element={
                <RequireAdmin>
                  <MockAdminPage />
                </RequireAdmin>
              }
            />
          </Routes>
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByText('Public Page')).toBeInTheDocument();
      });
    });

    it('renders admin content when authenticated as admin', async () => {
      authStore.setUser({ did: 'did:test:admin', role: 'admin' });

      render(
        <MemoryRouter initialEntries={['/admin']}>
          <Routes>
            <Route path="/account/login" element={<MockLoginPage />} />
            <Route path="/" element={<MockPublicPage />} />
            <Route
              path="/admin"
              element={
                <RequireAdmin>
                  <MockAdminPage />
                </RequireAdmin>
              }
            />
          </Routes>
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByText('Admin Page')).toBeInTheDocument();
      });
    });
  });
});
