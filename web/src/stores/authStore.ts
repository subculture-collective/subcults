/**
 * Auth Store
 * Authentication state management with token refresh and multi-tab synchronization
 */

import { useState, useEffect, useCallback } from 'react';
import { apiClient } from '../lib/api-client';

export interface User {
  did: string;
  role: 'user' | 'admin';
}

interface AuthState {
  user: User | null;
  isAuthenticated: boolean;
  isAdmin: boolean;
  isLoading: boolean;
  accessToken: string | null;
}

// BroadcastChannel for multi-tab synchronization
const LOGOUT_CHANNEL = 'subcults_auth_logout';
let logoutChannel: BroadcastChannel | null = null;

// Initialize BroadcastChannel if supported
if (typeof BroadcastChannel !== 'undefined') {
  logoutChannel = new BroadcastChannel(LOGOUT_CHANNEL);
}

// In-memory auth state (access token stored here, refresh token in httpOnly cookie)
let authState: AuthState = {
  user: null,
  isAuthenticated: false,
  isAdmin: false,
  isLoading: true, // Start as loading until we check for existing session
  accessToken: null,
};

const listeners = new Set<(state: AuthState) => void>();

// Exponential backoff configuration
const RETRY_CONFIG = {
  maxRetries: 3,
  initialDelay: 1000, // 1 second
  maxDelay: 10000, // 10 seconds
};

/**
 * Sleep utility for retry delays
 */
const sleep = (ms: number): Promise<void> => {
  return new Promise(resolve => setTimeout(resolve, ms));
};

/**
 * Refresh access token using refresh token (stored in httpOnly cookie)
 * Implements exponential backoff for transient failures
 */
const refreshAccessToken = async (
  retryCount = 0
): Promise<string | null> => {
  try {
    // Call refresh endpoint (refresh token sent automatically via httpOnly cookie)
    const response = await fetch('/api/auth/refresh', {
      method: 'POST',
      credentials: 'include', // Include cookies
    });

    if (!response.ok) {
      // Don't retry on 401 (refresh token invalid/expired)
      if (response.status === 401) {
        return null;
      }

      // Retry on transient errors (5xx, network issues)
      if (response.status >= 500 && retryCount < RETRY_CONFIG.maxRetries) {
        const delay = Math.min(
          RETRY_CONFIG.initialDelay * Math.pow(2, retryCount),
          RETRY_CONFIG.maxDelay
        );
        await sleep(delay);
        return refreshAccessToken(retryCount + 1);
      }

      return null;
    }

    const data = await response.json();
    
    // Update access token in memory
    authState.accessToken = data.accessToken;
    authState.user = data.user;
    authState.isAuthenticated = true;
    authState.isAdmin = data.user.role === 'admin';
    authState.isLoading = false;
    
    notifyListeners();
    
    return data.accessToken;
  } catch (error) {
    // Retry on network errors
    if (retryCount < RETRY_CONFIG.maxRetries) {
      const delay = Math.min(
        RETRY_CONFIG.initialDelay * Math.pow(2, retryCount),
        RETRY_CONFIG.maxDelay
      );
      await sleep(delay);
      return refreshAccessToken(retryCount + 1);
    }

    console.error('Token refresh failed:', error);
    return null;
  }
};

/**
 * Handle unauthorized state (refresh failed)
 */
const handleUnauthorized = (): void => {
  // Clear auth state
  authState = {
    user: null,
    isAuthenticated: false,
    isAdmin: false,
    isLoading: false,
    accessToken: null,
  };
  
  notifyListeners();
  
  // Broadcast logout to other tabs
  if (logoutChannel) {
    logoutChannel.postMessage({ type: 'logout' });
  }
  
  // Redirect to login if not already there
  if (window.location.pathname !== '/account/login') {
    window.location.href = '/account/login';
  }
};

/**
 * Notify all listeners of state changes
 */
const notifyListeners = (): void => {
  listeners.forEach((listener) => listener(authState));
};

// Initialize API client with auth callbacks
apiClient.initialize({
  baseURL: '/api',
  getAccessToken: () => authState.accessToken,
  refreshToken: refreshAccessToken,
  onUnauthorized: handleUnauthorized,
});

// Listen for logout events from other tabs
if (logoutChannel) {
  logoutChannel.addEventListener('message', (event) => {
    if (event.data.type === 'logout') {
      // Another tab logged out, sync this tab
      authState = {
        user: null,
        isAuthenticated: false,
        isAdmin: false,
        isLoading: false,
        accessToken: null,
      };
      notifyListeners();
      
      // Redirect to login if not already there
      if (window.location.pathname !== '/account/login') {
        window.location.href = '/account/login';
      }
    }
  });
}

export const authStore = {
  getState: (): AuthState => authState,

  subscribe: (listener: (state: AuthState) => void): (() => void) => {
    listeners.add(listener);
    return () => {
      listeners.delete(listener);
    };
  },

  /**
   * Set user and access token (called after successful login)
   */
  setUser: (user: User, accessToken: string): void => {
    authState = {
      user,
      isAuthenticated: true,
      isAdmin: user.role === 'admin',
      isLoading: false,
      accessToken,
    };
    notifyListeners();
  },

  /**
   * Logout user and clear tokens
   * Refresh token is cleared by calling backend logout endpoint
   */
  logout: async (): Promise<void> => {
    try {
      // Call logout endpoint to clear refresh token cookie
      await fetch('/api/auth/logout', {
        method: 'POST',
        credentials: 'include',
      });
    } catch (error) {
      console.error('Logout request failed:', error);
      // Continue with local logout even if request fails
    }

    // Clear local state
    authState = {
      user: null,
      isAuthenticated: false,
      isAdmin: false,
      isLoading: false,
      accessToken: null,
    };
    notifyListeners();

    // Broadcast logout to other tabs
    if (logoutChannel) {
      logoutChannel.postMessage({ type: 'logout' });
    }
  },

  /**
   * Initialize auth state by checking for existing session
   * Should be called on app startup
   */
  initialize: async (): Promise<void> => {
    try {
      // Try to refresh token to check if we have a valid session
      const token = await refreshAccessToken();
      
      if (!token) {
        // No valid session
        authState.isLoading = false;
        notifyListeners();
      }
    } catch (error) {
      console.error('Auth initialization failed:', error);
      authState.isLoading = false;
      notifyListeners();
    }
  },
};

/**
 * React hook for accessing auth state
 */
export const useAuth = () => {
  const [state, setState] = useState(() => authStore.getState());

  useEffect(() => {
    const unsubscribe = authStore.subscribe(setState);
    return unsubscribe;
  }, []);

  // Provide logout function that components can call
  const logout = useCallback(async () => {
    await authStore.logout();
  }, []);

  return {
    ...state,
    logout,
  };
};
