import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createMemoryRouter, RouterProvider } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { SearchResultsPage } from './SearchResultsPage';
import { apiClient } from '../lib/api-client';
import { getAppearances } from '../lib/release-api';

vi.mock('../lib/api-client', () => ({ apiClient: { searchScenes: vi.fn(), searchEvents: vi.fn(), searchPosts: vi.fn() } }));
vi.mock('../lib/release-api', () => ({ getAppearances: vi.fn() }));

const scenes = [{ id: 's1', name: 'Techno Underground', description: 'Chicago collective', allow_precise: false, coarse_geohash: 'dp3w' }];
const events = [{ id: 'e1', scene_id: 's1', name: 'Friday Night Rave', description: 'All night dancing', allow_precise: false }];
const appearances = [{ id: 'a1', event: { id: 'e1', title: 'Friday Night Rave', starts_at: '2026-08-10T20:00:00Z', kind: 'show', occurrence: { coarse_geohash: 'dp3w', precision: 'coarse' } }, act: { id: 'a', profile_id: 'p1', name: 'Night System' }, tour: { id: 't1', title: 'Signal Path Tour' }, host_names: [], context: 'tour_stop', locality: 'visiting', status: 'announced', verification: 'verified' }];

function renderPage(search = '') {
  const router = createMemoryRouter([{ path: '/search', element: <SearchResultsPage /> }], { initialEntries: [`/search${search}`], future: { v7_startTransition: true, v7_relativeSplatPath: true } });
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(<QueryClientProvider client={client}><RouterProvider router={router}/></QueryClientProvider>);
}

describe('SearchResultsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(apiClient.searchScenes).mockResolvedValue(scenes);
    vi.mocked(apiClient.searchEvents).mockResolvedValue(events);
    vi.mocked(apiClient.searchPosts).mockResolvedValue([]);
    vi.mocked(getAppearances).mockResolvedValue(appearances as never);
  });

  it('browses artists and tours without a query', async () => {
    renderPage('?type=artists');
    expect(await screen.findByRole('link', { name: /Night System/i })).toHaveAttribute('href', '/profiles/p1');
    expect(screen.queryByText('Signal Path Tour')).not.toBeInTheDocument();
  });

  it('shows scene and event results for a query and never renders posts', async () => {
    renderPage('?q=techno');
    expect(await screen.findByRole('link', { name: /Techno Underground/i })).toHaveAttribute('href', '/scenes/s1');
    expect(screen.getByRole('link', { name: /Friday Night Rave/i })).toHaveAttribute('href', '/events/e1');
    expect(screen.queryByText(/post/i)).not.toBeInTheDocument();
  });

  it('changes the URL-backed type filter', async () => {
    renderPage('?q=techno');
    const artists = screen.getByRole('button', { name: 'artists' });
    await userEvent.click(artists);
    expect(artists).toHaveAttribute('aria-pressed', 'true');
  });

  it('shows a clear empty state', async () => {
    vi.mocked(apiClient.searchScenes).mockResolvedValue([]); vi.mocked(apiClient.searchEvents).mockResolvedValue([]); vi.mocked(getAppearances).mockResolvedValue([]);
    renderPage('?q=missing');
    expect(await screen.findByText(/No public records match/i)).toBeInTheDocument();
  });

  it('keeps semantic landmarks', async () => {
    renderPage();
    expect(screen.getByRole('main')).toBeInTheDocument();
    expect(screen.getByRole('complementary', { name: 'Search filters' })).toBeInTheDocument();
    await waitFor(() => expect(screen.queryByRole('status')).not.toBeInTheDocument());
  });
});
