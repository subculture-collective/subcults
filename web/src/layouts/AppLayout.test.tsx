/**
 * AppLayout tests
 * Tests for layout structure and accessibility features
 */

import { describe, it, expect, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { createMemoryRouter, RouterProvider } from 'react-router-dom';
import { AppLayout } from './AppLayout';
import { authStore } from '../stores/authStore';

describe('AppLayout', () => {
  beforeEach(() => {
    authStore.resetForTesting();
  });

  it('renders header with logo', () => {
    const router = createMemoryRouter(
      [
        {
          path: '/',
          element: <AppLayout />,
        },
      ],
      {
        future: {
          v7_startTransition: true,
          v7_relativeSplatPath: true,
        },
      }
    );

    render(<RouterProvider router={router} />);

    expect(screen.getByRole('link', { name: 'Subcults discovery home' })).toBeInTheDocument();
  });

  it('renders skip-to-content link', () => {
    const router = createMemoryRouter(
      [
        {
          path: '/',
          element: <AppLayout />,
        },
      ],
      {
        future: {
          v7_startTransition: true,
          v7_relativeSplatPath: true,
        },
      }
    );

    render(<RouterProvider router={router} />);

    const skipLink = screen.getByText('Skip to content');
    expect(skipLink).toBeInTheDocument();
    expect(skipLink.getAttribute('href')).toBe('#main-content');
  });

  it('renders main content area with proper landmarks', () => {
    const router = createMemoryRouter(
      [
        {
          path: '/',
          element: <AppLayout />,
        },
      ],
      {
        future: {
          v7_startTransition: true,
          v7_relativeSplatPath: true,
        },
      }
    );

    const { container } = render(<RouterProvider router={router} />);

    expect(container.querySelector('header')).toBeInTheDocument();
    expect(container.querySelector('main#main-content')).toBeInTheDocument();
    expect(container.querySelector('footer')).toBeInTheDocument();
  });

  it('shows login button when not authenticated', () => {
    const router = createMemoryRouter(
      [
        {
          path: '/',
          element: <AppLayout />,
        },
      ],
      {
        future: {
          v7_startTransition: true,
          v7_relativeSplatPath: true,
        },
      }
    );

    render(<RouterProvider router={router} />);

    expect(screen.getAllByRole('link', { name: 'Sign in' }).length).toBeGreaterThan(0);
  });

  it('shows user info and logout when authenticated', async () => {
    authStore.setUser({ did: 'did:example:test-user-12345', handle: 'listener', role: 'user' }, 'test-token');

    const router = createMemoryRouter(
      [
        {
          path: '/',
          element: <AppLayout />,
        },
      ],
      {
        future: {
          v7_startTransition: true,
          v7_relativeSplatPath: true,
        },
      }
    );

    render(<RouterProvider router={router} />);
    expect(screen.getByRole('link', { name: '@listener' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Sign out' })).toBeInTheDocument();
  });

  it('shows Studio for an approved creator', () => {
    authStore.setUser({ did: 'did:example:creator', role: 'creator' }, 'test-token');

    const router = createMemoryRouter(
      [
        {
          path: '/',
          element: <AppLayout />,
        },
      ],
      {
        future: {
          v7_startTransition: true,
          v7_relativeSplatPath: true,
        },
      }
    );

    render(<RouterProvider router={router} />);
    expect(screen.getByRole('link', { name: 'Studio' })).toBeInTheDocument();
  });

  it('does not show Studio for regular users', () => {
    authStore.setUser({ did: 'did:example:user', role: 'user' }, 'test-token');

    const router = createMemoryRouter(
      [
        {
          path: '/',
          element: <AppLayout />,
        },
      ],
      {
        future: {
          v7_startTransition: true,
          v7_relativeSplatPath: true,
        },
      }
    );

    render(<RouterProvider router={router} />);
    expect(screen.queryByRole('link', { name: 'Studio' })).not.toBeInTheDocument();
  });

  it('renders navigation with proper aria labels', () => {
    const router = createMemoryRouter(
      [
        {
          path: '/',
          element: <AppLayout />,
        },
      ],
      {
        future: {
          v7_startTransition: true,
          v7_relativeSplatPath: true,
        },
      }
    );

    const { container } = render(<RouterProvider router={router} />);

    expect(container.querySelector('nav[aria-label="Primary navigation"]')).toBeInTheDocument();
    expect(container.querySelector('nav[aria-label="Mobile shortcuts"]')).toBeInTheDocument();
  });
});
