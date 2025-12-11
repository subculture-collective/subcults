/**
 * ConnectionIndicator Component Tests
 */

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ConnectionIndicator } from './ConnectionIndicator';

describe('ConnectionIndicator', () => {
  it('renders excellent quality', () => {
    render(<ConnectionIndicator quality="excellent" />);

    const status = screen.getByRole('status');
    expect(status).toHaveAttribute('aria-label');
    expect(status.getAttribute('aria-label')).toMatch(/streaming\.connectionIndicator/i);
    expect(screen.getByText(/streaming\.connectionIndicator\.excellent/i)).toBeInTheDocument();
  });

  it('renders good quality', () => {
    render(<ConnectionIndicator quality="good" />);

    const status = screen.getByRole('status');
    expect(status).toHaveAttribute('aria-label');
    expect(status.getAttribute('aria-label')).toMatch(/streaming\.connectionIndicator/i);
    expect(screen.getByText(/streaming\.connectionIndicator\.good/i)).toBeInTheDocument();
  });

  it('renders poor quality', () => {
    render(<ConnectionIndicator quality="poor" />);

    const status = screen.getByRole('status');
    expect(status).toHaveAttribute('aria-label');
    expect(status.getAttribute('aria-label')).toMatch(/streaming\.connectionIndicator/i);
    expect(screen.getByText(/streaming\.connectionIndicator\.poor/i)).toBeInTheDocument();
  });

  it('renders unknown quality', () => {
    render(<ConnectionIndicator quality="unknown" />);

    const status = screen.getByRole('status');
    expect(status).toHaveAttribute('aria-label');
    expect(status.getAttribute('aria-label')).toMatch(/streaming\.connectionIndicator/i);
    expect(screen.getByText(/streaming\.connectionIndicator\.unknown/i)).toBeInTheDocument();
  });

  it('hides label when showLabel is false', () => {
    render(<ConnectionIndicator quality="excellent" showLabel={false} />);

    expect(screen.queryByText(/streaming\.connectionIndicator\.excellent/i)).not.toBeInTheDocument();
    // Status role should still be present
    expect(screen.getByRole('status')).toBeInTheDocument();
  });

  it('shows label by default', () => {
    render(<ConnectionIndicator quality="good" />);

    expect(screen.getByText(/streaming\.connectionIndicator\.good/i)).toBeInTheDocument();
  });

  it('renders signal bars', () => {
    const { container } = render(<ConnectionIndicator quality="excellent" />);

    // Should have 3 signal bars
    const bars = container.querySelectorAll('.signal-bar');
    expect(bars).toHaveLength(3);
  });
});
