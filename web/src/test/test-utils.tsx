/* eslint-disable react-refresh/only-export-components */

/**
 * Test utilities
 * Common test helpers and setup
 */

import { createMemoryRouter, RouterProvider } from 'react-router-dom';

/* eslint-disable @typescript-eslint/no-explicit-any */

/**
 * Create a memory router with v7 future flags enabled
 * This prevents "Future Flag Warning" messages in tests
 */
export function createTestRouter(routes: any[], initialEntries?: string[]) {
  return createMemoryRouter(routes, {
    initialEntries,
    future: {
      v7_startTransition: true,
      v7_relativeSplatPath: true,
    },
  });
}

/**
 * Wrapper component for tests using createTestRouter
 */
export function TestRouterProvider({ router }: { router: ReturnType<typeof createTestRouter> }) {
  return <RouterProvider router={router} />;
}
