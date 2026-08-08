import type { ReactNode } from 'react';
import { Navigate, useLocation } from 'react-router-dom';
import { useAuth } from '../stores/authStore';

export function RequireCreator({ children }: { children: ReactNode }) {
  const auth = useAuth();
  const location = useLocation();
  if (auth.isLoading) return <div className="content-wrap py-16" aria-busy="true"><p className="eyebrow">Resolving creator authority…</p></div>;
  if (!auth.isAuthenticated) return <Navigate replace to="/login" state={{ from: location }} />;
  if (!auth.isCreator) return <Navigate replace to="/creator-access" />;
  return children;
}
