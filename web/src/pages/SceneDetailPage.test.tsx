import { QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import { createMemoryRouter, RouterProvider } from 'react-router-dom';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { createTestQueryClient } from '../test/test-utils';
import { SceneDetailPage } from './SceneDetailPage';

const scene = {
  id: 'scene-1',
  name: 'South Side Frequencies',
  description: 'Independent electronic music across Chicago.',
  allow_precise: false,
  coarse_geohash: 'dp3wj',
  visibility: 'public',
  tags: ['electronic', 'all-ages'],
};

function renderPage(sceneID = 'scene-1') {
  const router = createMemoryRouter([{ path: '/scenes/:id', element: <SceneDetailPage /> }], {
    initialEntries: [`/scenes/${sceneID}`],
    future: { v7_startTransition: true, v7_relativeSplatPath: true },
  });
  return render(<QueryClientProvider client={createTestQueryClient()}><RouterProvider router={router} /></QueryClientProvider>);
}

describe('SceneDetailPage', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('renders the resolved scene and disclosure-safe location label', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: async () => scene }));
    renderPage();
    expect(await screen.findByRole('heading', { level: 1, name: scene.name })).toBeInTheDocument();
    expect(screen.getByText(scene.description)).toBeInTheDocument();
    expect(screen.getByText('Coarse community area')).toBeInTheDocument();
  });

  it('uses the route identifier when loading a scene', async () => {
    const fetchMock = vi.fn().mockResolvedValue({ ok: true, json: async () => ({ ...scene, id: 'abc123' }) });
    vi.stubGlobal('fetch', fetchMock);
    renderPage('abc123');
    await screen.findByRole('heading', { name: scene.name });
    expect(fetchMock).toHaveBeenCalledWith('/api/scenes/abc123', expect.objectContaining({ credentials: 'include' }));
  });

  it('renders tags and the related-date discovery link', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: async () => scene }));
    renderPage();
    await screen.findByText('electronic');
    expect(screen.getByText('all-ages')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Search related dates' })).toHaveAttribute('href', '/search?q=South%20Side%20Frequencies&type=events');
  });

  it('renders the unavailable state for a missing scene', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, json: async () => ({}) }));
    renderPage('missing');
    expect(await screen.findByRole('heading', { name: 'Signal not found.' })).toBeInTheDocument();
  });
});
