import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { ConsentControl } from './ConsentControl';

const consent = {
  status: 'not_granted' as const,
  verification_state: 'verified' as const,
  scope: {
    id: 'scope-1',
    sender: { id: 'profile-1', name: 'Oracle Sisters', type: 'profile' },
    channel: 'email',
    purpose: 'tour announcements',
    disclosure_version: '2026-08',
    frequency: 'Up to two per month',
    region: 'US',
    tour: { id: 'tour-1', name: 'Autumn Dates' },
    place: { id: 'place-1', name: 'The Echo' },
  },
};

describe('ConsentControl', () => {
  it('renders the complete scope and keeps verification separate from consent', () => {
    render(<ConsentControl consent={consent} onChange={vi.fn()} />);

    expect(screen.getByText('Oracle Sisters')).toBeInTheDocument();
    expect(screen.getByText('tour announcements')).toBeInTheDocument();
    expect(screen.getByText('Up to two per month')).toBeInTheDocument();
    expect(screen.getByText('2026-08')).toBeInTheDocument();
    expect(screen.getByText('Autumn Dates')).toBeInTheDocument();
    expect(screen.getByText('The Echo')).toBeInTheDocument();
    expect(screen.getByText('verified')).toBeInTheDocument();
    expect(screen.getByText(/RSVPs, and membership do not grant delivery consent/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Grant email consent' })).toBeEnabled();
    expect(screen.getByRole('button', { name: 'Revoke email consent' })).toBeDisabled();
  });

  it('calls the explicit grant action', async () => {
    const onChange = vi.fn().mockResolvedValue(undefined);
    render(<ConsentControl consent={consent} onChange={onChange} />);

    await userEvent.setup().click(screen.getByRole('button', { name: 'Grant email consent' }));

    expect(onChange).toHaveBeenCalledWith(consent.scope, 'grant');
  });

  it('enables an explicit revoke only for granted consent', () => {
    render(<ConsentControl consent={{ ...consent, status: 'granted' }} onChange={vi.fn()} />);

    expect(screen.getByRole('button', { name: 'Grant email consent' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Revoke email consent' })).toBeEnabled();
  });
});
