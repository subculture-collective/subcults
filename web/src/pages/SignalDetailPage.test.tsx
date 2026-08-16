import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { QueryClientProvider } from '@tanstack/react-query';
import { createTestQueryClient } from '../test/test-utils';
import { SignalDetailPage } from './SignalDetailPage';

const detail = {
  signal: {
    id: 'signal-1',
    title: 'Chicago show added',
    body: 'An all-ages date has been added.',
    state: 'published',
    sender: { id: 'profile-1', name: 'Oracle Sisters', type: 'profile' },
    target: { id: 'tour-1', type: 'tour', title: 'Autumn Dates' },
  },
  consent_scopes: [{
    status: 'not_granted',
    verification_state: 'verified',
    scope: {
      id: 'scope-1',
      sender: { id: 'profile-1', name: 'Oracle Sisters', type: 'profile' },
      channel: 'email',
      purpose: 'tour announcements',
      disclosure_version: '2026-08',
      frequency: 'Up to two per month',
      region: 'US',
      place: { id: 'place-1', name: 'Empty Bottle' },
    },
  }],
};

function renderPage() {
  return render(
    <QueryClientProvider client={createTestQueryClient()}><MemoryRouter initialEntries={['/signals/signal-1']}>
      <Routes><Route path="/signals/:id" element={<SignalDetailPage />} /></Routes>
    </MemoryRouter></QueryClientProvider>
  );
}

describe('SignalDetailPage', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('loads a Signal and renders its scoped delivery choice', async () => {
    const fetchMock = vi.fn().mockResolvedValue({ ok: true, json: async () => detail });
    vi.stubGlobal('fetch', fetchMock);
    renderPage();

    expect(await screen.findByRole('heading', { name: 'Chicago show added' })).toBeInTheDocument();
    expect(screen.getByText('From Oracle Sisters')).toBeInTheDocument();
    expect(screen.getByText('Empty Bottle')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Grant email consent' })).toBeEnabled();
    expect(fetchMock).toHaveBeenCalledWith('/api/signals/signal-1', expect.objectContaining({ credentials: 'include' }));
  });

  it('reports an unavailable Signal when the request fails', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false }));
    renderPage();

    expect(await screen.findByRole('heading', { name: 'Transmission not found.' })).toBeInTheDocument();
  });

  it('posts an explicit grant mutation and updates the visible state', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce({ ok: true, json: async () => detail })
      .mockResolvedValueOnce({ ok: true, json: async () => ({ consent: { ...detail.consent_scopes[0], status: 'granted' } }) });
    vi.stubGlobal('fetch', fetchMock);
    renderPage();

    await screen.findByRole('button', { name: 'Grant email consent' });
    await userEvent.setup().click(screen.getByRole('button', { name: 'Grant email consent' }));

    await waitFor(() => expect(fetchMock).toHaveBeenLastCalledWith('/api/audience/consent', expect.objectContaining({
      method: 'POST', body: JSON.stringify({ scope_id: 'scope-1', action: 'grant' }),
    })));
    await waitFor(() => expect(screen.getByRole('button', { name: 'Revoke email consent' })).toBeEnabled());
  });
});
