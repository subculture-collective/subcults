/**
 * Toast Store
 * Manages ephemeral toast notifications
 */

import { create } from 'zustand';

export type ToastType = 'success' | 'error' | 'info';

export interface Toast {
  id: string;
  type: ToastType;
  message: string;
  duration?: number; // milliseconds, undefined means no auto-dismiss
  dismissible?: boolean; // whether user can manually dismiss
}

interface ToastState {
  toasts: Toast[];
}

interface ToastActions {
  addToast: (toast: Omit<Toast, 'id'>) => string;
  removeToast: (id: string) => void;
  clearAll: () => void;
}

export type ToastStore = ToastState & ToastActions;

/**
 * Default toast configuration
 */
const DEFAULT_DURATION = 5000; // 5 seconds
const DEFAULT_DISMISSIBLE = true;

/**
 * Generate unique toast ID
 */
function generateToastId(): string {
  return `toast-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
}

/**
 * Toast notification store
 */
export const useToastStore = create<ToastStore>((set) => ({
  toasts: [],

  addToast: (toast) => {
    const id = generateToastId();
    const newToast: Toast = {
      id,
      type: toast.type,
      message: toast.message,
      duration: toast.duration ?? DEFAULT_DURATION,
      dismissible: toast.dismissible ?? DEFAULT_DISMISSIBLE,
    };

    set((state) => ({
      toasts: [...state.toasts, newToast],
    }));

    // Auto-dismiss if duration is specified
    if (newToast.duration !== undefined && newToast.duration > 0) {
      setTimeout(() => {
        set((state) => ({
          toasts: state.toasts.filter((t) => t.id !== id),
        }));
      }, newToast.duration);
    }

    return id;
  },

  removeToast: (id) => {
    set((state) => ({
      toasts: state.toasts.filter((t) => t.id !== id),
    }));
  },

  clearAll: () => {
    set({ toasts: [] });
  },
}));

/**
 * Hook for toast notifications
 * Provides convenient methods for common toast types
 */
export function useToasts() {
  const { addToast, removeToast, clearAll } = useToastStore();

  return {
    success: (message: string, duration?: number) => 
      addToast({ type: 'success', message, duration }),
    
    error: (message: string, duration?: number) => 
      addToast({ type: 'error', message, duration }),
    
    info: (message: string, duration?: number) => 
      addToast({ type: 'info', message, duration }),
    
    dismiss: removeToast,
    
    clearAll,

    // Advanced: add custom toast with full control
    custom: addToast,
  };
}
