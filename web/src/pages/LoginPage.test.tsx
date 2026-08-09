import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { LoginPage } from './LoginPage';
import { requestMagicLink } from '../lib/auth-service';

vi.mock('../lib/auth-service', () => ({ requestMagicLink: vi.fn() }));
function renderLogin() { return render(<MemoryRouter><LoginPage/></MemoryRouter>); }

describe('LoginPage', () => {
  beforeEach(() => vi.clearAllMocks());
  it('presents a passwordless email form', () => {
    renderLogin();
    expect(screen.getByRole('heading', { name: 'Sign in to Subcult' })).toBeInTheDocument();
    expect(screen.getByLabelText('Email address')).toHaveAttribute('autocomplete', 'email');
    expect(screen.getByRole('button', { name: 'Email me a sign-in link' })).toBeInTheDocument();
    expect(screen.queryByLabelText(/password/i)).not.toBeInTheDocument();
  });
  it('requests a one-time link and shows the privacy-preserving success state', async () => {
    vi.mocked(requestMagicLink).mockResolvedValue(undefined);
    renderLogin();
    await userEvent.type(screen.getByLabelText('Email address'), 'fan@example.com');
    await userEvent.click(screen.getByRole('button', { name: 'Email me a sign-in link' }));
    expect(requestMagicLink).toHaveBeenCalledWith('fan@example.com', '/me');
    expect(await screen.findByText(/If that address can receive mail/i)).toBeInTheDocument();
  });
  it('shows service errors accessibly', async () => {
    vi.mocked(requestMagicLink).mockRejectedValue(new Error('Email service unavailable'));
    renderLogin();
    await userEvent.type(screen.getByLabelText('Email address'), 'fan@example.com');
    await userEvent.click(screen.getByRole('button', { name: 'Email me a sign-in link' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('Email service unavailable');
  });
});
