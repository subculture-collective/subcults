import { QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { expectNoA11yViolations } from '../test/a11y-helpers';
import { createTestQueryClient } from '../test/test-utils';
import { SceneDetailPage } from './SceneDetailPage';

function renderScene() {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: async () => ({
    id: 'test-scene-123', name: 'Test Scene', description: 'A readable scene description.',
    allow_precise: false, coarse_geohash: 'dp3wj', visibility: 'public', tags: ['noise'],
  }) }));
  return render(<QueryClientProvider client={createTestQueryClient()}><MemoryRouter initialEntries={['/scenes/test-scene-123']}>
    <Routes><Route path="/scenes/:id" element={<SceneDetailPage />} /></Routes>
  </MemoryRouter></QueryClientProvider>);
}

describe('SceneDetailPage - Accessibility', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('has no detectable accessibility violations', async () => {
    const { container } = renderScene();
    await screen.findByRole('heading', { level: 1, name: 'Test Scene' });
    await expectNoA11yViolations(container);
  });

  it('uses a single descriptive level-one heading and a following section heading', async () => {
    renderScene();
    expect(await screen.findByRole('heading', { level: 1, name: 'Test Scene' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { level: 2, name: /Dates connected/ })).toBeInTheDocument();
  });

  it('keeps descriptive content and the location disclosure readable', async () => {
    renderScene();
    expect(await screen.findByText('A readable scene description.')).toBeInTheDocument();
    expect(screen.getByText('Coarse community area')).toBeInTheDocument();
  });
});
