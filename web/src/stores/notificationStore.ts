/**
 * Notification Store
 * Manages Web Push notification subscription state with localStorage persistence
 */

import { create } from 'zustand';

export type NotificationPermission = 'default' | 'granted' | 'denied';

export interface PushSubscriptionData {
  endpoint: string;
  keys: {
    p256dh: string;
    auth: string;
  };
}

interface NotificationState {
  permission: NotificationPermission;
  isSubscribed: boolean;
  subscription: PushSubscriptionData | null;
  isLoading: boolean;
  error: string | null;
}

interface NotificationActions {
  setPermission: (permission: NotificationPermission) => void;
  setSubscription: (subscription: PushSubscriptionData | null) => void;
  setIsSubscribed: (isSubscribed: boolean) => void;
  setLoading: (isLoading: boolean) => void;
  setError: (error: string | null) => void;
  reset: () => void;
}

export type NotificationStore = NotificationState & NotificationActions;

/**
 * Local storage key for subscription state
 */
const NOTIFICATION_STORAGE_KEY = 'subcults-notification-subscription';

/**
 * Get initial subscription state from localStorage
 */
function getStoredSubscription(): PushSubscriptionData | null {
  try {
    const stored = localStorage.getItem(NOTIFICATION_STORAGE_KEY);
    if (stored) {
      return JSON.parse(stored);
    }
  } catch (error) {
    console.warn('[notificationStore] Failed to parse stored subscription:', error);
  }
  return null;
}

/**
 * Persist subscription to localStorage
 */
function persistSubscription(subscription: PushSubscriptionData | null): void {
  try {
    if (subscription) {
      localStorage.setItem(NOTIFICATION_STORAGE_KEY, JSON.stringify(subscription));
    } else {
      localStorage.removeItem(NOTIFICATION_STORAGE_KEY);
    }
  } catch (error) {
    console.warn('[notificationStore] Failed to persist subscription:', error);
  }
}

/**
 * Get initial notification permission
 */
function getInitialPermission(): NotificationPermission {
  if (typeof Notification === 'undefined') {
    return 'default';
  }
  return Notification.permission as NotificationPermission;
}

/**
 * Notification store for managing Web Push subscriptions
 */
export const useNotificationStore = create<NotificationStore>((set) => ({
  permission: getInitialPermission(),
  isSubscribed: false,
  subscription: getStoredSubscription(),
  isLoading: false,
  error: null,

  setPermission: (permission: NotificationPermission) => {
    set({ permission });
  },

  setSubscription: (subscription: PushSubscriptionData | null) => {
    set({ subscription, isSubscribed: subscription !== null });
    persistSubscription(subscription);
  },

  setIsSubscribed: (isSubscribed: boolean) => {
    set({ isSubscribed });
  },

  setLoading: (isLoading: boolean) => {
    set({ isLoading });
  },

  setError: (error: string | null) => {
    set({ error });
  },

  reset: () => {
    set({
      permission: getInitialPermission(),
      isSubscribed: false,
      subscription: null,
      isLoading: false,
      error: null,
    });
    persistSubscription(null);
  },
}));

/**
 * Hook for notification state only (optimized for re-renders)
 */
export function useNotificationState() {
  const permission = useNotificationStore((state) => state.permission);
  const isSubscribed = useNotificationStore((state) => state.isSubscribed);
  const subscription = useNotificationStore((state) => state.subscription);
  const isLoading = useNotificationStore((state) => state.isLoading);
  const error = useNotificationStore((state) => state.error);
  
  return {
    permission,
    isSubscribed,
    subscription,
    isLoading,
    error,
  };
}

/**
 * Hook for notification actions only (stable reference)
 */
export function useNotificationActions() {
  const setPermission = useNotificationStore((state) => state.setPermission);
  const setSubscription = useNotificationStore((state) => state.setSubscription);
  const setIsSubscribed = useNotificationStore((state) => state.setIsSubscribed);
  const setLoading = useNotificationStore((state) => state.setLoading);
  const setError = useNotificationStore((state) => state.setError);
  const reset = useNotificationStore((state) => state.reset);
  
  return {
    setPermission,
    setSubscription,
    setIsSubscribed,
    setLoading,
    setError,
    reset,
  };
}
