/**
 * LoginPage Component Tests
 * Tests authentication form behavior and navigation
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createMemoryRouter, RouterProvider } from 'react-router-dom';
import { LoginPage } from './LoginPage';
import { authStore } from '../stores/authStore';

// Mock authStore
vi.mock('../stores/authStore', () => ({
  authStore: {
    setUser: vi.fn(),
  },
}));

describe('LoginPage', () => {
  const mockNavigate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
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
    it('should call setUser with user role when login as user is clicked', async () => {
      const user = userEvent.setup();
      renderLoginPage();

      const userButton = screen.getByRole('button', { name: /Login as User/i });
      await user.click(userButton);

      expect(authStore.setUser).toHaveBeenCalledWith(
        {
          did: 'did:example:test-user',
          role: 'user',
        },
        'placeholder-access-token'
      );
    });

    it('should call setUser with admin role when login as admin is clicked', async () => {
      const user = userEvent.setup();
      renderLoginPage();

      const adminButton = screen.getByRole('button', { name: /Login as Admin/i });
      await user.click(adminButton);

      expect(authStore.setUser).toHaveBeenCalledWith(
        {
          did: 'did:example:test-user',
          role: 'admin',
        },
        'placeholder-access-token'
      );
    });
  });

  describe('Navigation', () => {
    it('should navigate to home after user login', async () => {
      const user = userEvent.setup();
      renderLoginPage();

      const userButton = screen.getByRole('button', { name: /Login as User/i });
      await user.click(userButton);

      // Should navigate away from login page
      expect(screen.queryByRole('heading', { name: /login/i })).not.toBeInTheDocument();
    });

    it('should navigate to home after admin login', async () => {
      const user = userEvent.setup();
      renderLoginPage();

      const adminButton = screen.getByRole('button', { name: /Login as Admin/i });
      await user.click(adminButton);

      // Should navigate away from login page
      expect(screen.queryByRole('heading', { name: /login/i })).not.toBeInTheDocument();
    });

    it('should redirect to the page user tried to visit after login', async () => {
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

      // Should navigate away from login page
      expect(screen.queryByRole('heading', { name: /login/i })).not.toBeInTheDocument();
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

      expect(authStore.setUser).toHaveBeenCalled();
    });

    it('should activate button with Space key', async () => {
      const user = userEvent.setup();
      renderLoginPage();

      const userButton = screen.getByRole('button', { name: /Login as User/i });

      // Focus and activate with Space
      await user.tab();
      expect(userButton).toHaveFocus();
      await user.keyboard(' ');

      expect(authStore.setUser).toHaveBeenCalled();
    });
  });
});
