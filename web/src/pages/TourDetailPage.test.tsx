import { render, screen, waitFor } from '@testing-library/react';
import { createMemoryRouter, RouterProvider } from 'react-router-dom';
import { describe, expect, it, vi } from 'vitest';
import { TourDetailPage } from './TourDetailPage';
vi.mock('../components/MapView', () => ({ MapView: () => <div data-testid="tour-map" /> }));
describe('TourDetailPage', () => { it('renders a tour title and occurrence map surface', async () => {
  vi.mocked(fetch).mockResolvedValueOnce({ ok: true, json: async () => ({ tour: { id: 't', title: 'Signal Run' }, appearances: [] }) } as Response);
  const router = createMemoryRouter([{ path: '/tours/:id', element: <TourDetailPage /> }], { initialEntries: ['/tours/t'] }); render(<RouterProvider router={router} />);
  await waitFor(() => expect(screen.getByRole('heading', { name: 'Signal Run' })).toBeInTheDocument()); expect(screen.getByTestId('tour-map')).toBeInTheDocument();
}); });
