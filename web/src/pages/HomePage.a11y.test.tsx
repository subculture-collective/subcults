import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { HomePage } from './HomePage';
import { expectNoA11yViolations } from '../test/a11y-helpers';

vi.mock('../lib/release-api', () => ({ getAppearances: vi.fn().mockResolvedValue([]) }));
vi.mock('../components/MapView', () => ({ MapView: () => <div role="application" aria-label="Interactive map showing public events" data-testid="map-container"/> }));
vi.mock('../components/discovery/TourMapLayer', () => ({ TourMapLayer: () => null }));

function renderHome() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(<QueryClientProvider client={client}><MemoryRouter><HomePage/></MemoryRouter></QueryClientProvider>);
}

describe('HomePage accessibility', () => {
  it('has no automated accessibility violations', async () => { const { container } = renderHome(); await expectNoA11yViolations(container); });
  it('exposes filter state and an accessible map', async () => {
    renderHome();
    const tonight = screen.getByRole('button', { name: 'Tonight' });
    await userEvent.click(tonight); expect(tonight).toHaveAttribute('aria-pressed', 'true');
    await userEvent.click(screen.getByRole('button', { name: 'Map' }));
    expect(screen.getByRole('application', { name: 'Interactive map showing public events' })).toBeInTheDocument();
  });
});
