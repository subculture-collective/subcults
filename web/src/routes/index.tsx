/**
 * Route Configuration
 * Defines all application routes with lazy loading and guards
 */

import React, { Suspense, lazy } from 'react';
import { createBrowserRouter, RouterProvider } from 'react-router-dom';
import { AppLayout } from '../layouts/AppLayout';
import { LoadingSkeleton } from '../components/LoadingSkeleton';
import { ErrorBoundary } from '../components/ErrorBoundary';
import { RequireAuth } from '../guards/RequireAuth';
import { RequireAdmin } from '../guards/RequireAdmin';

// Eagerly loaded pages
import { HomePage } from '../pages/HomePage';
import { SceneDetailPage } from '../pages/SceneDetailPage';
import { EventDetailPage } from '../pages/EventDetailPage';
import { AccountPage } from '../pages/AccountPage';
import { LoginPage } from '../pages/LoginPage';
import { SettingsPage } from '../pages/SettingsPage';
import { NotFoundPage } from '../pages/NotFoundPage';

// Lazy loaded pages (heavy dependencies)
const StreamPage = lazy(() =>
  import('../pages/StreamPage').then((module) => ({ default: module.StreamPage }))
);
const AdminPage = lazy(() =>
  import('../pages/AdminPage').then((module) => ({ default: module.AdminPage }))
);
const StreamingDemo = lazy(() =>
  import('../StreamingDemo').then((module) => ({ default: module.StreamingDemo }))
);

/**
 * Router configuration
 * Routes are organized by access level and loading strategy
 */
const router = createBrowserRouter(
  [
    {
      path: '/',
      element: <AppLayout />,
      errorElement: (
        <ErrorBoundary>
          <NotFoundPage />
        </ErrorBoundary>
      ),
      children: [
        // Public routes
        {
          index: true,
          element: <HomePage />,
        },
        {
          path: 'scenes/:id',
          element: <SceneDetailPage />,
        },
        {
          path: 'events/:id',
          element: <EventDetailPage />,
        },
        {
          path: 'account/login',
          element: <LoginPage />,
        },
        {
          path: 'demo/streaming',
          element: (
            <Suspense fallback={<LoadingSkeleton />}>
              <StreamingDemo />
            </Suspense>
          ),
        },

        // Protected routes (require authentication)
        {
          path: 'account',
          element: (
            <RequireAuth>
              <AccountPage />
            </RequireAuth>
          ),
        },
        {
          path: 'settings',
          element: (
            <RequireAuth>
              <SettingsPage />
            </RequireAuth>
          ),
        },
        {
          path: 'stream/:room',
          element: (
            <RequireAuth>
              <Suspense fallback={<LoadingSkeleton />}>
                <StreamPage />
              </Suspense>
            </RequireAuth>
          ),
        },

        // Admin routes (require admin role)
        {
          path: 'admin',
          element: (
            <RequireAdmin>
              <Suspense fallback={<LoadingSkeleton />}>
                <AdminPage />
              </Suspense>
            </RequireAdmin>
          ),
        },

        // 404 catch-all
        {
          path: '*',
          element: <NotFoundPage />,
        },
      ],
    },
  ],
  {
    future: {
      v7_startTransition: true,
      v7_relativeSplatPath: true,
    },
  }
);

/**
 * AppRouter Component
 * Provides routing context to the application
 */
export const AppRouter: React.FC = () => {
  return <RouterProvider router={router} />;
};
