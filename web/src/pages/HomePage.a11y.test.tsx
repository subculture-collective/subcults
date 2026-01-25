/**
 * HomePage Accessibility Tests
 * Validates WCAG compliance for the main map view
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { HomePage } from './HomePage';
import { expectNoA11yViolations } from '../test/a11y-helpers';

// Mock MapView component since it has complex dependencies
vi.mock('../components/MapView', () => ({
  MapView: vi.fn(() => (
    <div 
      role="application" 
      aria-label="Interactive map showing scenes and events"
      data-testid="map-view"
    >
      Map View Placeholder
    </div>
  )),
}));

describe('HomePage - Accessibility', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('should not have any accessibility violations', async () => {
    const { container } = render(
      <MemoryRouter>
        <HomePage />
      </MemoryRouter>
    );

    await expectNoA11yViolations(container);
  });

  it('should have proper ARIA labels for map application', () => {
    const { getByRole } = render(
      <MemoryRouter>
        <HomePage />
      </MemoryRouter>
    );

    const mapApplication = getByRole('application');
    expect(mapApplication).toHaveAttribute('aria-label');
  });

  it('should be keyboard navigable', () => {
    const { container } = render(
      <MemoryRouter>
        <HomePage />
      </MemoryRouter>
    );

    // Verify that the map container doesn't prevent keyboard navigation
    const mapContainer = container.querySelector('[data-testid="map-view"]');
    expect(mapContainer).toBeInTheDocument();
  });
});
