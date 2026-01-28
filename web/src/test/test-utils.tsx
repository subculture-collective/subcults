/**
 * Test utilities
 * Common test helpers and setup
 */

import { ReactNode } from 'react';
import { createMemoryRouter, RouterProvider } from 'react-router-dom';

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
export function TestRouterProvider({
  router,
  children,
}: {
  router: ReturnType<typeof createTestRouter>;
  children?: ReactNode;
}) {
  return <RouterProvider router={router} />;
}
