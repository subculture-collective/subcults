import { render, screen } from '@testing-library/react';
import { createMemoryRouter, RouterProvider } from 'react-router-dom';
import { describe, expect, it } from 'vitest';
import { AppearanceCard } from './AppearanceCard';
import type { AppearanceSummary } from '../../types/touring';

const visitingFixture: AppearanceSummary = { id: 'appearance-1', event: { id: 'event-1', title: 'Metro, Chicago', starts_at: '2026-09-01T20:00:00Z', kind: 'show', occurrence: { coarse_geohash: 'dp3', precision: 'coarse' } }, act: { id: 'act-1', name: 'Circuit', home_territory: 'Detroit' }, host_names: ['Smartbar'], context: 'tour_stop', locality: 'visiting', status: 'confirmed', verification: 'verified' };

describe('AppearanceCard', () => {
  it('shows host, occurrence, and visiting context separately', () => {
    const router = createMemoryRouter([{ path: '*', element: <AppearanceCard appearance={visitingFixture} /> }], { initialEntries: ['/'] });
    render(<RouterProvider router={router} />);
    expect(screen.getByText('Metro, Chicago')).toBeInTheDocument();
    expect(screen.getByText('Hosted by Smartbar')).toBeInTheDocument();
    expect(screen.getByText('Visiting from Detroit')).toBeInTheDocument();
  });
});
