/**
 * ToastContainer Component
 * Displays toast notifications with accessibility support
 */

import { useToastStore, type Toast, type ToastType } from '../stores/toastStore';

/**
 * Get icon for toast type
 */
function getToastIcon(type: ToastType): string {
  switch (type) {
    case 'success':
      return '✓';
    case 'error':
      return '✕';
    case 'info':
      return 'ℹ';
  }
}

/**
 * Get background color for toast type
 */
function getToastColor(type: ToastType): string {
  switch (type) {
    case 'success':
      return '#10b981'; // green-500
    case 'error':
      return '#ef4444'; // red-500
    case 'info':
      return '#3b82f6'; // blue-500
  }
}

/**
 * Individual toast notification
 */
function ToastItem({ toast, onDismiss }: { toast: Toast; onDismiss: (id: string) => void }) {
  return (
    <div
      role="status"
      aria-live="polite"
      aria-atomic="true"
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: '0.75rem',
        padding: '1rem',
        backgroundColor: getToastColor(toast.type),
        color: 'white',
        borderRadius: '0.5rem',
        boxShadow: '0 4px 6px -1px rgb(0 0 0 / 0.1), 0 2px 4px -2px rgb(0 0 0 / 0.1)',
        minWidth: '300px',
        maxWidth: '500px',
        animation: 'slideIn 0.3s ease-out',
      }}
    >
      <div
        style={{
          fontSize: '1.25rem',
          fontWeight: 'bold',
          flexShrink: 0,
        }}
        aria-hidden="true"
      >
        {getToastIcon(toast.type)}
      </div>
      <div
        style={{
          flex: 1,
          fontSize: '0.875rem',
          lineHeight: '1.25rem',
        }}
      >
        {toast.message}
      </div>
      {toast.dismissible && (
        <button
          onClick={() => onDismiss(toast.id)}
          aria-label="Dismiss notification"
          style={{
            background: 'transparent',
            border: 'none',
            color: 'white',
            cursor: 'pointer',
            padding: '0.25rem',
            fontSize: '1.25rem',
            lineHeight: 1,
            opacity: 0.8,
            flexShrink: 0,
          }}
          onMouseEnter={(e) => (e.currentTarget.style.opacity = '1')}
          onMouseLeave={(e) => (e.currentTarget.style.opacity = '0.8')}
        >
          ×
        </button>
      )}
    </div>
  );
}

// Animation keyframes - defined once at module level
const TOAST_ANIMATIONS = `
  @keyframes slideIn {
    from {
      transform: translateX(100%);
      opacity: 0;
    }
    to {
      transform: translateX(0);
      opacity: 1;
    }
  }
`;

/**
 * Toast container component
 * Renders all active toasts in a fixed position
 */
export function ToastContainer() {
  const toasts = useToastStore((state) => state.toasts);
  const removeToast = useToastStore((state) => state.removeToast);

  if (toasts.length === 0) {
    return null;
  }

  return (
    <>
      <style>{TOAST_ANIMATIONS}</style>
      <div
        aria-label="Notifications"
        role="region"
        aria-live="polite"
        style={{
          position: 'fixed',
          top: '1rem',
          right: '1rem',
          display: 'flex',
          flexDirection: 'column',
          gap: '0.75rem',
          zIndex: 9999,
          pointerEvents: 'none',
        }}
      >
        {toasts.map((toast) => (
          <div key={toast.id} style={{ pointerEvents: 'auto' }}>
            <ToastItem toast={toast} onDismiss={removeToast} />
          </div>
        ))}
      </div>
    </>
  );
}
