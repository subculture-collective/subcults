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
    
    // Check for spinner div with animation styles
    const spinner = container.querySelector('div[style*="animation"]');
    expect(spinner).toBeInTheDocument();
  });

  it('applies correct layout styles', () => {
    const { container } = render(<LoadingSkeleton />);
    
    const loadingSkeleton = container.querySelector('.loading-skeleton');
    expect(loadingSkeleton).toBeInTheDocument();
    expect(loadingSkeleton).toHaveStyle({
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      height: '100vh',
      width: '100%',
    });
  });

  it('has dark background', () => {
    const { container } = render(<LoadingSkeleton />);
    
    const loadingSkeleton = container.querySelector('.loading-skeleton');
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
    const { container } = render(<LoadingSkeleton />);
    
    // Check the inner div (child of loading-skeleton) has center text alignment
    const loadingSkeleton = container.querySelector('.loading-skeleton');
    const contentWrapper = loadingSkeleton?.children[0] as HTMLElement;
    expect(contentWrapper).toBeTruthy();
    const style = contentWrapper?.getAttribute('style') || '';
    expect(style).toContain('text-align: center');
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
    // Check that border styles are present
    expect(style).toContain('border-width: 4px');
    expect(style).toContain('border-style: solid');
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
