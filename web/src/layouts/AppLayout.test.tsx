/**
 * AppLayout tests
 * Tests for layout structure and accessibility features
 */

import { describe, it, expect, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { AppLayout } from './AppLayout';
import { authStore } from '../stores/authStore';

describe('AppLayout', () => {
  beforeEach(() => {
    authStore.logout();
  });

  it('renders header with logo', () => {
    render(
      <MemoryRouter>
        <AppLayout />
      </MemoryRouter>
    );

    expect(screen.getByText('Subcults')).toBeInTheDocument();
  });

  it('renders skip-to-content link', () => {
    render(
      <MemoryRouter>
        <AppLayout />
      </MemoryRouter>
    );

    const skipLink = screen.getByText('Skip to content');
    expect(skipLink).toBeInTheDocument();
    expect(skipLink.getAttribute('href')).toBe('#main-content');
  });

  it('renders main content area with proper landmarks', () => {
    const { container } = render(
      <MemoryRouter>
        <AppLayout />
      </MemoryRouter>
    );

    // Check for semantic HTML elements
    expect(container.querySelector('header[role="banner"]')).toBeInTheDocument();
    expect(container.querySelector('main[role="main"]')).toBeInTheDocument();
    expect(container.querySelector('nav[role="navigation"]')).toBeInTheDocument();
  });

  it('shows login button when not authenticated', () => {
    render(
      <MemoryRouter>
        <AppLayout />
      </MemoryRouter>
    );

    expect(screen.getByText('Login')).toBeInTheDocument();
  });

  it('shows user info and logout when authenticated', () => {
    authStore.setUser({ did: 'did:example:test-user-12345', role: 'user' });

    render(
      <MemoryRouter>
        <AppLayout />
      </MemoryRouter>
    );

    // DID is truncated with ellipsis in the UI
    expect(screen.getByText(/did:example:test-use/)).toBeInTheDocument();
    expect(screen.getByText('Logout')).toBeInTheDocument();
  });

  it('shows admin link when user is admin', () => {
    authStore.setUser({ did: 'did:example:admin', role: 'admin' });

    render(
      <MemoryRouter>
        <AppLayout />
      </MemoryRouter>
    );

    const adminLinks = screen.getAllByText('Admin');
    expect(adminLinks.length).toBeGreaterThan(0);
  });

  it('does not show admin link for regular users', () => {
    authStore.setUser({ did: 'did:example:user', role: 'user' });

    render(
      <MemoryRouter>
        <AppLayout />
      </MemoryRouter>
    );

    expect(screen.queryByText('Admin')).not.toBeInTheDocument();
  });

  it('renders navigation with proper aria labels', () => {
    render(
      <MemoryRouter>
        <AppLayout />
      </MemoryRouter>
    );

    expect(
      screen.getByRole('navigation', { name: 'Main navigation' })
    ).toBeInTheDocument();
  });
});
