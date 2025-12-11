/**
 * ConnectionIndicator Component Tests
 */

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ConnectionIndicator } from './ConnectionIndicator';

describe('ConnectionIndicator', () => {
  it('renders excellent quality', () => {
    render(<ConnectionIndicator quality="excellent" />);

    expect(screen.getByRole('status')).toHaveAttribute(
      'aria-label',
      'Connection quality: Excellent'
    );
    expect(screen.getByText(/excellent/i)).toBeInTheDocument();
  });

  it('renders good quality', () => {
    render(<ConnectionIndicator quality="good" />);

    expect(screen.getByRole('status')).toHaveAttribute(
      'aria-label',
      'Connection quality: Good'
    );
    expect(screen.getByText(/good/i)).toBeInTheDocument();
  });

  it('renders poor quality', () => {
    render(<ConnectionIndicator quality="poor" />);

    expect(screen.getByRole('status')).toHaveAttribute(
      'aria-label',
      'Connection quality: Poor'
    );
    expect(screen.getByText(/poor/i)).toBeInTheDocument();
  });

  it('renders unknown quality', () => {
    render(<ConnectionIndicator quality="unknown" />);

    expect(screen.getByRole('status')).toHaveAttribute(
      'aria-label',
      'Connection quality: Unknown'
    );
    expect(screen.getByText(/unknown/i)).toBeInTheDocument();
  });

  it('hides label when showLabel is false', () => {
    render(<ConnectionIndicator quality="excellent" showLabel={false} />);

    expect(screen.queryByText(/excellent/i)).not.toBeInTheDocument();
    // Status role should still be present
    expect(screen.getByRole('status')).toBeInTheDocument();
  });

  it('shows label by default', () => {
    render(<ConnectionIndicator quality="good" />);

    expect(screen.getByText(/good/i)).toBeInTheDocument();
  });

  it('renders signal bars', () => {
    const { container } = render(<ConnectionIndicator quality="excellent" />);

    // Should have 3 signal bars
    const bars = container.querySelectorAll('.signal-bar');
    expect(bars).toHaveLength(3);
  });
});
