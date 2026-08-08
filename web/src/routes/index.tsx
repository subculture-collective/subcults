import { Suspense, lazy } from 'react';
import { Navigate, RouterProvider, createBrowserRouter } from 'react-router-dom';
import { LoadingSkeleton } from '../components/LoadingSkeleton';
import { AppLayout } from '../layouts/AppLayout';
import { RequireAuth } from '../guards/RequireAuth';
import { RequireAdmin } from '../guards/RequireAdmin';
import { RequireCreator } from '../guards/RequireCreator';

const pages = {
  home: lazy(() => import('../pages/HomePage').then((m) => ({ default: m.HomePage }))),
  scenes: lazy(() => import('../pages/ScenesPage').then((m) => ({ default: m.ScenesPage }))),
  scene: lazy(() => import('../pages/SceneDetailPage').then((m) => ({ default: m.SceneDetailPage }))),
  events: lazy(() => import('../pages/EventsPage').then((m) => ({ default: m.EventsPage }))),
  event: lazy(() => import('../pages/EventDetailPage').then((m) => ({ default: m.EventDetailPage }))),
  profile: lazy(() => import('../pages/ProfileDetailPage').then((m) => ({ default: m.ProfileDetailPage }))),
  tour: lazy(() => import('../pages/TourDetailPage').then((m) => ({ default: m.TourDetailPage }))),
  signal: lazy(() => import('../pages/SignalDetailPage').then((m) => ({ default: m.SignalDetailPage }))),
  search: lazy(() => import('../pages/SearchResultsPage').then((m) => ({ default: m.SearchResultsPage }))),
  login: lazy(() => import('../pages/LoginPage').then((m) => ({ default: m.LoginPage }))),
  verify: lazy(() => import('../pages/AuthVerifyPage').then((m) => ({ default: m.AuthVerifyPage }))),
  onboarding: lazy(() => import('../pages/OnboardingPage').then((m) => ({ default: m.OnboardingPage }))),
  me: lazy(() => import('../pages/MyActivityPage').then((m) => ({ default: m.MyActivityPage }))),
  creatorAccess: lazy(() => import('../pages/CreatorAccessPage').then((m) => ({ default: m.CreatorAccessPage }))),
  studio: lazy(() => import('../pages/StudioPage').then((m) => ({ default: m.StudioPage }))),
  admin: lazy(() => import('../pages/AdminPage').then((m) => ({ default: m.AdminPage }))),
  legal: lazy(() => import('../pages/LegalPage').then((m) => ({ default: m.LegalPage }))),
  notFound: lazy(() => import('../pages/NotFoundPage').then((m) => ({ default: m.NotFoundPage }))),
};

const load = (element: React.ReactNode) => <Suspense fallback={<LoadingSkeleton />}>{element}</Suspense>;
const protect = (element: React.ReactNode) => <RequireAuth>{load(element)}</RequireAuth>;

// Exported for focused routing contract tests.
// eslint-disable-next-line react-refresh/only-export-components
export const router = createBrowserRouter([{
  path: '/',
  element: <AppLayout />,
  children: [
    { index: true, element: load(<pages.home />) },
    { path: 'scenes', element: load(<pages.scenes />) },
    { path: 'scenes/:id', element: load(<pages.scene />) },
    { path: 'events', element: load(<pages.events />) },
    { path: 'events/:id', element: load(<pages.event />) },
    { path: 'profiles/:id', element: load(<pages.profile />) },
    { path: 'tours/:id', element: load(<pages.tour />) },
    { path: 'signals/:id', element: load(<pages.signal />) },
    { path: 'search', element: load(<pages.search />) },
    { path: 'login', element: load(<pages.login />) },
    { path: 'account/login', element: <Navigate replace to="/login" /> },
    { path: 'auth/verify', element: load(<pages.verify />) },
    { path: 'onboarding', element: protect(<pages.onboarding />) },
    { path: 'me', element: protect(<pages.me />) },
    { path: 'account', element: <Navigate replace to="/me" /> },
    { path: 'settings', element: <Navigate replace to="/me" /> },
    { path: 'creator-access', element: load(<pages.creatorAccess />) },
    { path: 'studio/*', element: <RequireCreator>{load(<pages.studio />)}</RequireCreator> },
    { path: 'admin/*', element: <RequireAdmin>{load(<pages.admin />)}</RequireAdmin> },
    { path: 'privacy', element: load(<pages.legal />) },
    { path: 'terms', element: load(<pages.legal />) },
    { path: '*', element: load(<pages.notFound />) },
  ],
}], { future: { v7_relativeSplatPath: true } });

export function AppRouter() { return <RouterProvider router={router} />; }
