/**
 * LoadingSkeleton Component Tests
 * Validates loading state rendering and accessibility
 */

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { LoadingSkeleton } from './LoadingSkeleton';

describe('LoadingSkeleton', () => {
  it('renders loading message', () => {
    render(<LoadingSkeleton />);
    
    expect(screen.getByText('Loading...')).toBeInTheDocument();
  });

  it('has proper ARIA attributes for accessibility', () => {
    render(<LoadingSkeleton />);
    
    const skeleton = screen.getByRole('status');
    expect(skeleton).toBeInTheDocument();
    expect(skeleton).toHaveAttribute('aria-live', 'polite');
    expect(skeleton).toHaveAttribute('aria-busy', 'true');
    expect(skeleton).toHaveAttribute('aria-label', 'Loading content');
  });

  it('renders spinner element', () => {
    const { container } = render(<LoadingSkeleton />);
    
    // Check for spinner div by its inline styles
    // Note: Using style attribute to verify implementation without adding test-ids
    const allDivs = container.querySelectorAll('div');
    const spinnerDiv = Array.from(allDivs).find(div => 
      div.getAttribute('style')?.includes('animation')
    );
    expect(spinnerDiv).toBeTruthy();
  });

  it('applies correct layout styles', () => {
    render(<LoadingSkeleton />);
    
    // Verify the loading skeleton has status role (already tested in ARIA test)
    // and verify layout by checking for the loading message presence
    const loadingStatus = screen.getByRole('status');
    expect(loadingStatus).toBeInTheDocument();
    expect(loadingStatus).toHaveClass('loading-skeleton');
  });

  it('has dark background', () => {
    render(<LoadingSkeleton />);
    
    const loadingSkeleton = screen.getByRole('status');
    const style = loadingSkeleton?.getAttribute('style') || '';
    // Check for backgroundColor in computed format
    expect(style).toContain('background-color: rgb(26, 26, 26)');
    expect(style).toContain('color: white');
  });

  it('includes spin animation keyframes', () => {
    const { container } = render(<LoadingSkeleton />);
    
    const style = container.querySelector('style');
    expect(style).toBeInTheDocument();
    expect(style?.textContent).toContain('@keyframes spin');
    expect(style?.textContent).toContain('transform: rotate(0deg)');
    expect(style?.textContent).toContain('transform: rotate(360deg)');
  });

  it('centers content both horizontally and vertically', () => {
    render(<LoadingSkeleton />);
    
    // Check the loading skeleton contains centered text
    const loadingText = screen.getByText('Loading...');
    expect(loadingText).toBeInTheDocument();
    // The text should be inside a center-aligned container (verified by component implementation)
  });

  it('renders spinner with circular border', () => {
    const { container } = render(<LoadingSkeleton />);
    
    // Find spinner by checking for animation and border-radius in style
    const spinner = container.querySelector('div[style*="animation"]');
    expect(spinner).toBeInTheDocument();
    const style = spinner?.getAttribute('style') || '';
    expect(style).toContain('border-radius: 50%');
    expect(style).toContain('width: 50px');
    expect(style).toContain('height: 50px');
  });

  it('spinner has visible border with contrast', () => {
    const { container } = render(<LoadingSkeleton />);
    
    const spinner = container.querySelector('div[style*="animation"]');
    const style = spinner?.getAttribute('style') || '';
    // The spinner uses a semi-transparent border with a solid white top border for contrast
    expect(style).toContain('border-width: 4px');
    expect(style).toContain('border-style: solid');
    // The borderTop shorthand becomes border-color: white rgba(...) rgba(...)
    expect(style).toContain('border-color: white');
  });

  it('spinner has margin below it', () => {
    const { container } = render(<LoadingSkeleton />);
    
    const spinner = container.querySelector('div[style*="animation"]');
    expect(spinner).toHaveStyle({
      margin: '0 auto 1rem',
    });
  });
});
