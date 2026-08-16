import { useCallback, useEffect, useState } from 'react';
import { apiClient } from '../lib/api-client';

export type UserRole = 'user' | 'participant' | 'creator_pending' | 'creator' | 'admin';

export interface User {
  id?: string;
  did: string;
  handle?: string;
  display_name?: string;
  role: UserRole;
  onboarding_complete?: boolean;
}

interface AuthState {
  user: User | null;
  isAuthenticated: boolean;
  isAdmin: boolean;
  isCreator: boolean;
  isLoading: boolean;
  accessToken: string | null;
}

interface AuthPayload {
  access_token: string;
  user: User;
  return_path?: string;
}

const configuredAPI = (import.meta.env.VITE_API_URL || '/api').replace(/\/$/, '');
export const API_V1 = configuredAPI.endsWith('/v1') ? configuredAPI : `${configuredAPI}/v1`;

let state: AuthState = {
  user: null,
  isAuthenticated: false,
  isAdmin: false,
  isCreator: false,
  isLoading: true,
  accessToken: null,
};

const listeners = new Set<(next: AuthState) => void>();
const channel = typeof BroadcastChannel === 'undefined' ? null : new BroadcastChannel('subcult-auth');
let refreshPromise: Promise<string | null> | null = null;
const refreshBackoffMs = [250, 500, 1000];

function publish(next: AuthState) {
  state = next;
  listeners.forEach((listener) => listener(state));
}

function authenticated(user: User, accessToken: string): AuthState {
  return {
    user,
    accessToken,
    isAuthenticated: true,
    isAdmin: user.role === 'admin',
    isCreator: user.role === 'creator' || user.role === 'admin',
    isLoading: false,
  };
}

function anonymous(isLoading = false): AuthState {
  return { user: null, accessToken: null, isAuthenticated: false, isAdmin: false, isCreator: false, isLoading };
}

async function refreshAccessToken(): Promise<string | null> {
  if (refreshPromise) return refreshPromise;
  refreshPromise = (async () => {
    try {
      for (let attempt = 0; attempt <= refreshBackoffMs.length; attempt += 1) {
        const response = await fetch(`${API_V1}/auth/refresh`, { method: 'POST', credentials: 'include' });
        if (response.ok) {
          const body = await response.json() as { data: AuthPayload };
          publish(authenticated(body.data.user, body.data.access_token));
          return body.data.access_token;
        }
        if (response.status < 500 || attempt === refreshBackoffMs.length) {
          if (response.status !== 401) {
            console.warn(`[authStore] Token refresh failed with status ${response.status}`);
          }
          publish(anonymous());
          return null;
        }
        await new Promise((resolve) => setTimeout(resolve, refreshBackoffMs[attempt]));
      }
      return null;
    } catch {
      publish(anonymous());
      return null;
    } finally {
      refreshPromise = null;
    }
  })();
  return refreshPromise;
}

apiClient.initialize({
  baseURL: API_V1,
  getAccessToken: () => state.accessToken,
  refreshToken: refreshAccessToken,
  onUnauthorized: () => publish(anonymous()),
});

channel?.addEventListener('message', (event) => {
  if (event.data?.type === 'logout') publish(anonymous());
});

export const authStore = {
  getState: () => state,
  subscribe(listener: (next: AuthState) => void) {
    listeners.add(listener);
    return () => { listeners.delete(listener); };
  },
  setUser(user: User, accessToken: string) {
    publish(authenticated(user, accessToken));
  },
  async initialize() {
    if (!state.isLoading) return;
    await refreshAccessToken();
  },
  async logout() {
    try {
      await fetch(`${API_V1}/auth/logout`, { method: 'POST', credentials: 'include' });
    } catch {
      // Local logout must succeed even when the server is unreachable.
    } finally {
      publish(anonymous());
      channel?.postMessage({ type: 'logout' });
    }
  },
  resetForTesting() {
    publish(anonymous(true));
  },
};

export function useAuth() {
  const [snapshot, setSnapshot] = useState(() => authStore.getState());
  useEffect(() => authStore.subscribe(setSnapshot), []);
  const logout = useCallback(() => authStore.logout(), []);
  return { ...snapshot, logout };
}
