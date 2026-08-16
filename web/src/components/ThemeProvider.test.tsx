/**
 * ThemeProvider Tests
 * Validates theme initialization and system preference sync
 */

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { render } from '@testing-library/react';
import { ThemeProvider } from './ThemeProvider';
import { useThemeStore } from '../stores/themeStore';

describe('ThemeProvider', () => {
  let mockMediaQuery: {
    matches: boolean;
    media: string;
    addEventListener: ReturnType<typeof vi.fn>;
    removeEventListener: ReturnType<typeof vi.fn>;
    addListener: ReturnType<typeof vi.fn>;
    removeListener: ReturnType<typeof vi.fn>;
  };

  beforeEach(() => {
    localStorage.clear();
    useThemeStore.setState({ theme: 'light' });
    document.documentElement.classList.remove('dark');

    // Setup matchMedia mock
    mockMediaQuery = {
      matches: false,
      media: '(prefers-color-scheme: dark)',
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
    };

    Object.defineProperty(window, 'matchMedia', {
      writable: true,
      value: vi.fn().mockReturnValue(mockMediaQuery),
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('renders children', () => {
    const { getByText } = render(
      <ThemeProvider>
        <div>Test Content</div>
      </ThemeProvider>
    );

    expect(getByText('Test Content')).toBeInTheDocument();
  });

  it('initializes theme on mount', () => {
    localStorage.setItem('subcults-theme', 'dark');

    render(
      <ThemeProvider>
        <div>Test</div>
      </ThemeProvider>
    );

    expect(useThemeStore.getState().theme).toBe('dark');
    expect(document.documentElement.classList.contains('dark')).toBe(true);
  });

  it('does not keep a system-theme listener after initialization', () => {
    render(
      <ThemeProvider>
        <div>Test</div>
      </ThemeProvider>
    );

    expect(mockMediaQuery.addEventListener).not.toHaveBeenCalled();
  });

  it('does not require listener cleanup on unmount', () => {
    const { unmount } = render(
      <ThemeProvider>
        <div>Test</div>
      </ThemeProvider>
    );

    unmount();

    expect(mockMediaQuery.removeEventListener).not.toHaveBeenCalled();
  });

  it('initializes without using the legacy browser listener API', () => {
    // Remove modern addEventListener
    const mockMediaQueryLegacy = {
      ...mockMediaQuery,
      addEventListener: undefined,
    };
    
    Object.defineProperty(window, 'matchMedia', {
      writable: true,
      value: vi.fn().mockReturnValue(mockMediaQueryLegacy),
    });

    render(
      <ThemeProvider>
        <div>Test</div>
      </ThemeProvider>
    );

    expect(mockMediaQueryLegacy.addListener).not.toHaveBeenCalled();
  });

  it('does not require legacy listener cleanup', () => {
    const mockMediaQueryLegacy = {
      ...mockMediaQuery,
      addEventListener: undefined,
    };
    
    Object.defineProperty(window, 'matchMedia', {
      writable: true,
      value: vi.fn().mockReturnValue(mockMediaQueryLegacy),
    });
    
    const { unmount } = render(
      <ThemeProvider>
        <div>Test</div>
      </ThemeProvider>
    );

    unmount();

    expect(mockMediaQueryLegacy.removeListener).not.toHaveBeenCalled();
  });

  it('uses the system preference during initialization when no preference is stored', () => {
    localStorage.clear();
    mockMediaQuery.matches = true;
    
    render(
      <ThemeProvider>
        <div>Test</div>
      </ThemeProvider>
    );

    expect(useThemeStore.getState().theme).toBe('dark');
  });

  it('keeps a manually stored preference during initialization', () => {
    localStorage.setItem('subcults-theme', 'light');

    render(
      <ThemeProvider>
        <div>Test</div>
      </ThemeProvider>
    );

    expect(useThemeStore.getState().theme).toBe('light');
  });
});
