/**
 * LoginPage Component Tests
 * Tests authentication form behavior and navigation
 */

import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createMemoryRouter, RouterProvider } from 'react-router-dom';
import { LoginPage } from './LoginPage';
import { authStore } from '../stores/authStore';

// Mock navigate to test navigation behavior
const mockNavigate = vi.fn();
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

describe('LoginPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockNavigate.mockClear();
    // Reset auth state to logged out
    authStore.logout();
  });

  const renderLoginPage = (initialEntries = ['/login']) => {
    const router = createMemoryRouter(
      [
        {
          path: '/login',
          element: <LoginPage />,
        },
        {
          path: '/',
          element: <div>Home Page</div>,
        },
        {
          path: '/admin',
          element: <div>Admin Page</div>,
        },
      ],
      {
        initialEntries,
        future: {
          v7_startTransition: true,
          v7_relativeSplatPath: true,
        },
      }
    );

    return render(<RouterProvider router={router} />);
  };

  describe('Rendering', () => {
    it('should render login heading', () => {
      renderLoginPage();
      expect(screen.getByRole('heading', { name: /login/i })).toBeInTheDocument();
    });

    it('should render placeholder login message', () => {
      renderLoginPage();
      expect(
        screen.getByText(/Placeholder login - click to simulate authentication/i)
      ).toBeInTheDocument();
    });

    it('should render login as user button', () => {
      renderLoginPage();
      expect(screen.getByRole('button', { name: /Login as User/i })).toBeInTheDocument();
    });

    it('should render login as admin button', () => {
      renderLoginPage();
      expect(screen.getByRole('button', { name: /Login as Admin/i })).toBeInTheDocument();
    });
  });

  describe('User Authentication', () => {
    it('should authenticate user when login as user is clicked', async () => {
      const user = userEvent.setup();
      renderLoginPage();

      const userButton = screen.getByRole('button', { name: /Login as User/i });
      await user.click(userButton);

      // Verify auth state changed
      const state = authStore.getState();
      expect(state.isAuthenticated).toBe(true);
      expect(state.user?.role).toBe('user');
    });

    it('should authenticate admin when login as admin is clicked', async () => {
      const user = userEvent.setup();
      renderLoginPage();

      const adminButton = screen.getByRole('button', { name: /Login as Admin/i });
      await user.click(adminButton);

      // Verify auth state changed
      const state = authStore.getState();
      expect(state.isAuthenticated).toBe(true);
      expect(state.user?.role).toBe('admin');
      expect(state.isAdmin).toBe(true);
    });
  });

  describe('Navigation', () => {
    it('should navigate away after user login', async () => {
      const user = userEvent.setup();
      renderLoginPage();

      const userButton = screen.getByRole('button', { name: /Login as User/i });
      await user.click(userButton);

      // Verify navigation occurred
      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalled();
      });
    });

    it('should navigate away after admin login', async () => {
      const user = userEvent.setup();
      renderLoginPage();

      const adminButton = screen.getByRole('button', { name: /Login as Admin/i });
      await user.click(adminButton);

      // Verify navigation occurred
      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalled();
      });
    });

    it('should navigate to the intended page after login', async () => {
      const user = userEvent.setup();
      // Simulate user trying to visit /admin but being redirected to /login
      renderLoginPage([
        {
          pathname: '/login',
          state: { from: { pathname: '/admin' } },
        },
      ]);

      const adminButton = screen.getByRole('button', { name: /Login as Admin/i });
      await user.click(adminButton);

      // Should navigate to the originally requested page
      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith('/admin', { replace: true });
      });
    });
  });

  describe('Accessibility', () => {
    it('should have accessible buttons', () => {
      renderLoginPage();

      const userButton = screen.getByRole('button', { name: /Login as User/i });
      const adminButton = screen.getByRole('button', { name: /Login as Admin/i });

      expect(userButton).toBeEnabled();
      expect(adminButton).toBeEnabled();
    });

    it('should have proper heading hierarchy', () => {
      renderLoginPage();

      const heading = screen.getByRole('heading', { name: /login/i });
      expect(heading.tagName).toBe('H1');
    });
  });

  describe('Keyboard Navigation', () => {
    it('should allow keyboard navigation between buttons', async () => {
      const user = userEvent.setup();
      renderLoginPage();

      const userButton = screen.getByRole('button', { name: /Login as User/i });
      const adminButton = screen.getByRole('button', { name: /Login as Admin/i });

      // Tab to first button
      await user.tab();
      expect(userButton).toHaveFocus();

      // Tab to second button
      await user.tab();
      expect(adminButton).toHaveFocus();
    });

    it('should activate button with Enter key', async () => {
      const user = userEvent.setup();
      renderLoginPage();

      const userButton = screen.getByRole('button', { name: /Login as User/i });

      // Focus and activate with Enter
      await user.tab();
      expect(userButton).toHaveFocus();
      await user.keyboard('{Enter}');

      // Verify auth state changed
      await waitFor(() => {
        const state = authStore.getState();
        expect(state.isAuthenticated).toBe(true);
      });
    });

    it('should activate button with Space key', async () => {
      const user = userEvent.setup();
      renderLoginPage();

      const userButton = screen.getByRole('button', { name: /Login as User/i });

      // Focus and activate with Space
      await user.tab();
      expect(userButton).toHaveFocus();
      await user.keyboard(' ');

      // Verify auth state changed
      await waitFor(() => {
        const state = authStore.getState();
        expect(state.isAuthenticated).toBe(true);
      });
    });
  });
});
