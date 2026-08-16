/**
 * Test utilities
 * Common test helpers and setup
 */

import { createMemoryRouter, RouterProvider, RouteObject } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

export function createTestQueryClient() {
  return new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
}

/**
 * Create a memory router with v7 future flags enabled
 * This prevents "Future Flag Warning" messages in tests
 */
// eslint-disable-next-line react-refresh/only-export-components
export function createTestRouter(routes: RouteObject[], initialEntries?: string[]) {
  return createMemoryRouter(routes, {
    initialEntries,
    future: {
      v7_startTransition: true,
      v7_relativeSplatPath: true,
    },
  });
}

/**
 * Wrapper component props
 */
export interface TestRouterProviderProps {
  router: ReturnType<typeof createTestRouter>;
}

/**
 * Wrapper component for tests using createTestRouter
 */
export function TestRouterProvider({ router }: TestRouterProviderProps) {
  return <QueryClientProvider client={createTestQueryClient()}><RouterProvider router={router} /></QueryClientProvider>;
}
