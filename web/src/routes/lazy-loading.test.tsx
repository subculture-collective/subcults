import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { Outlet } from 'react-router-dom';

const SIMULATED_CHUNK_LOAD_MS = 25;

vi.mock('../layouts/AppLayout', () => ({
  AppLayout: () => <Outlet />,
}));

vi.mock('../pages/HomePage', async () => {
  await new Promise((resolve) => setTimeout(resolve, SIMULATED_CHUNK_LOAD_MS));
  return {
    HomePage: () => <div>Lazy Home Page</div>,
  };
});

describe('AppRouter lazy loading', () => {
  beforeEach(() => {
    window.history.pushState({}, '', '/');
  });

  it('shows loading skeleton while loading the home route chunk', async () => {
    const { AppRouter } = await import('./index');
    render(<AppRouter />);

    expect(screen.getByRole('status', { name: 'Loading content' })).toBeInTheDocument();

    await waitFor(() => {
      expect(screen.getByText('Lazy Home Page')).toBeInTheDocument();
    });
  });
});
