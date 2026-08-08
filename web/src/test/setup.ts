import { expect, afterEach, vi } from 'vitest';
import { cleanup } from '@testing-library/react';
import * as matchers from '@testing-library/jest-dom/matchers';
import { axe } from 'vitest-axe';
import * as axeMatchers from 'vitest-axe/matchers';

// Extend Vitest's expect with jest-dom matchers
expect.extend(matchers);

// Extend Vitest's expect with axe accessibility matchers
expect.extend(axeMatchers);

// Export axe for use in tests
export { axe };

// Node 25 exposes a process-level localStorage placeholder that is unavailable
// unless --localstorage-file is supplied. Provide the browser contract directly
// so component tests do not depend on a process-wide persistence file.
const storage = new (class implements Storage {
  private values = new Map<string, string>();

  get length() { return this.values.size; }
  clear() { this.values.clear(); }
  getItem(key: string) { return this.values.get(key) ?? null; }
  key(index: number) { return [...this.values.keys()][index] ?? null; }
  removeItem(key: string) { this.values.delete(key); }
  setItem(key: string, value: string) { this.values.set(key, String(value)); }
})();

Object.defineProperty(globalThis, 'localStorage', {
  configurable: true,
  value: storage,
});
Object.defineProperty(window, 'localStorage', {
  configurable: true,
  value: storage,
});

// Mock fetch globally for tests
// This prevents test failures when using relative URLs in authStore.logout()
// and supports i18next-http-backend which expects Response.text()
global.fetch = vi.fn(() => {
  // Mock response object with both json() and text() methods
  const mockResponse = {
    ok: true,
    status: 200,
    json: async () => ({}),
    text: async () => '{}', // Support i18next-http-backend
  } as Response;

  return Promise.resolve(mockResponse);
});

// Mock react-i18next
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: {
      language: 'en',
      changeLanguage: vi.fn(),
    },
  }),
  initReactI18next: {
    type: '3rdParty',
    init: vi.fn(),
  },
}));

// Mock IntersectionObserver for OptimizedImage component
global.IntersectionObserver = class IntersectionObserver {
  observe = vi.fn();
  disconnect = vi.fn();
  unobserve = vi.fn();
  
  constructor(callback: IntersectionObserverCallback) {
    // Immediately trigger intersection for all observed elements
    // Use queueMicrotask for predictable timing in tests
    queueMicrotask(() => {
      callback(
        [{ isIntersecting: true } as IntersectionObserverEntry],
        this as any
      );
    });
  }
} as any;

// Cleanup after each test
afterEach(() => {
  cleanup();
  // Reset fetch mock between tests
  vi.clearAllMocks();
});
