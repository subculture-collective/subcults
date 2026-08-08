import { render, screen, waitFor } from '@testing-library/react';
import { createMemoryRouter, RouterProvider } from 'react-router-dom';
import { describe, expect, it, vi } from 'vitest';
import { ProfileDetailPage } from './ProfileDetailPage';
describe('ProfileDetailPage', () => { it('renders the public home territory and appearances returned by the API', async () => {
  vi.mocked(fetch).mockResolvedValueOnce({ ok: true, json: async () => ({ profile: { id: 'p', name: 'Circuit', home_territory: 'Detroit' }, appearances: [] }) } as Response);
  const router = createMemoryRouter([{ path: '/profiles/:id', element: <ProfileDetailPage /> }], { initialEntries: ['/profiles/p'] }); render(<RouterProvider router={router} />);
  await waitFor(() => expect(screen.getByRole('heading', { name: 'Circuit' })).toBeInTheDocument()); expect(screen.getByText('Home territory: Detroit')).toBeInTheDocument();
}); });
