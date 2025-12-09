/**
 * Auth Store
 * Simple auth state management for route guards
 * This is a placeholder implementation until full auth is implemented
 */

export interface User {
  did: string;
  role: 'user' | 'admin';
}

interface AuthState {
  user: User | null;
  isAuthenticated: boolean;
  isAdmin: boolean;
}

// Simple in-memory store (will be replaced with proper state management)
let authState: AuthState = {
  user: null,
  isAuthenticated: false,
  isAdmin: false,
};

const listeners = new Set<(state: AuthState) => void>();

export const authStore = {
  getState: (): AuthState => authState,

  subscribe: (listener: (state: AuthState) => void): (() => void) => {
    listeners.add(listener);
    return () => {
      listeners.delete(listener);
    };
  },

  setUser: (user: User | null) => {
    authState = {
      user,
      isAuthenticated: !!user,
      isAdmin: user?.role === 'admin',
    };
    listeners.forEach((listener) => listener(authState));
  },

  logout: () => {
    authState = {
      user: null,
      isAuthenticated: false,
      isAdmin: false,
    };
    listeners.forEach((listener) => listener(authState));
  },
};

// Export React for the hook
import { useState, useEffect } from 'react';

/**
 * React hook for accessing auth state
 */
export const useAuth = () => {
  const [state, setState] = useState(authState);

  useEffect(() => {
    const unsubscribe = authStore.subscribe(setState);
    return unsubscribe;
  }, []);

  return state;
};
