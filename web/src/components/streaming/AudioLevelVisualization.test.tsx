/**
 * AudioLevelVisualization Component Tests
 */

import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { AudioLevelVisualization } from './AudioLevelVisualization';

// Mock react-i18next
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const translations: Record<string, string> = {
        'audioLevel.muted': 'Muted',
        'audioLevel.speaking': 'Speaking',
        'audioLevel.silent': 'Silent',
      };
      return translations[key] || key;
    },
  }),
}));

describe('AudioLevelVisualization', () => {
  it('renders 5 bars', () => {
    const { container } = render(<AudioLevelVisualization level={0.5} />);
    const bars = container.querySelectorAll('.audio-bar');
    expect(bars).toHaveLength(5);
  });

  it('shows muted label when muted', () => {
    render(<AudioLevelVisualization level={0.8} isMuted={true} />);
    expect(screen.getByRole('img', { name: 'Muted' })).toBeInTheDocument();
  });

  it('shows silent label when level is zero', () => {
    render(<AudioLevelVisualization level={0} />);
    expect(screen.getByRole('img', { name: 'Silent' })).toBeInTheDocument();
  });

  it('shows speaking label when showSpeaking and has level', () => {
    render(<AudioLevelVisualization level={0.5} showSpeaking={true} />);
    expect(screen.getByRole('img', { name: 'Speaking' })).toBeInTheDocument();
  });

  it('applies small size configuration', () => {
    const { container } = render(<AudioLevelVisualization level={0.5} size="small" />);
    const bars = container.querySelectorAll('.audio-bar');
    expect(bars[0]).toHaveStyle({ width: '3px' });
  });

  it('applies medium size configuration', () => {
    const { container } = render(<AudioLevelVisualization level={0.5} size="medium" />);
    const bars = container.querySelectorAll('.audio-bar');
    expect(bars[0]).toHaveStyle({ width: '4px' });
  });

  it('applies large size configuration', () => {
    const { container } = render(<AudioLevelVisualization level={0.5} size="large" />);
    const bars = container.querySelectorAll('.audio-bar');
    expect(bars[0]).toHaveStyle({ width: '5px' });
  });

  it('has proper accessibility role', () => {
    render(<AudioLevelVisualization level={0.5} />);
    expect(screen.getByRole('img')).toBeInTheDocument();
  });

  it('bars have aria-hidden for accessibility', () => {
    const { container } = render(<AudioLevelVisualization level={0.5} />);
    const bars = container.querySelectorAll('.audio-bar');
    bars.forEach((bar) => {
      expect(bar).toHaveAttribute('aria-hidden', 'true');
    });
  });

  it('all bars are inactive when muted', () => {
    const { container } = render(<AudioLevelVisualization level={0.9} isMuted={true} />);
    const inactiveBars = container.querySelectorAll('.audio-bar.inactive');
    expect(inactiveBars).toHaveLength(5);
  });

  it('has active and inactive bars based on level', () => {
    const { container } = render(<AudioLevelVisualization level={0.5} />);
    const activeBars = container.querySelectorAll('.audio-bar.active');
    const inactiveBars = container.querySelectorAll('.audio-bar.inactive');
    
    // At 0.5 level, we expect some active and some inactive bars
    expect(activeBars.length).toBeGreaterThan(0);
    expect(inactiveBars.length).toBeGreaterThan(0);
    expect(activeBars.length + inactiveBars.length).toBe(5);
  });

  it('all bars are inactive when level is zero', () => {
    const { container } = render(<AudioLevelVisualization level={0} />);
    const inactiveBars = container.querySelectorAll('.audio-bar.inactive');
    expect(inactiveBars).toHaveLength(5);
  });
});
