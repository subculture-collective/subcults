/**
 * RequireAdmin Guard
 * Protects admin-only routes
 * Redirects to home if user is not an admin
 */

import React from 'react';
import { Navigate } from 'react-router-dom';
import { useAuth } from '../stores/authStore';

interface RequireAdminProps {
  children: React.ReactNode;
}

export const RequireAdmin: React.FC<RequireAdminProps> = ({ children }) => {
  const { isAuthenticated, isAdmin } = useAuth();

  if (!isAuthenticated) {
    // Not authenticated - redirect to login
    return <Navigate to="/account/login" replace />;
  }

  if (!isAdmin) {
    // Authenticated but not admin - redirect to home
    return <Navigate to="/" replace />;
  }

  return <>{children}</>;
};
