import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import './i18n' // Initialize i18n
import App from './App.tsx'
import { initializeNotificationService } from './lib/notification-service'
import { errorLogger } from './lib/error-logger'
import { initPerformanceMonitoring } from './lib/performance-metrics'
import { registerServiceWorker } from './lib/service-worker'
import { useSettingsStore } from './stores/settingsStore'

// Global error handlers for uncaught errors and promise rejections
window.addEventListener('error', (event) => {
  errorLogger.logError(event.error || new Error(event.message));
});

window.addEventListener('unhandledrejection', (event) => {
  const error = event.reason instanceof Error 
    ? event.reason 
    : new Error(String(event.reason));
  errorLogger.logError(error);
});

// Register service worker for offline support and Web Push notifications
window.addEventListener('load', () => {
  registerServiceWorker({
    onUpdateInstalled: () => {
      // New service worker installed, prompt user to reload
      if (confirm('A new version of Subcults is available. Reload to update?')) {
        window.location.reload();
      }
    },
  });
});

// Initialize notification service with configuration
const VAPID_PUBLIC_KEY = import.meta.env.VITE_VAPID_PUBLIC_KEY;
const NOTIFICATION_API_ENDPOINT = '/api/notifications/subscribe';

// Fail fast in development if VAPID key is missing
if (import.meta.env.DEV && !VAPID_PUBLIC_KEY) {
  console.warn(
    '[NotificationService] VITE_VAPID_PUBLIC_KEY not set. Web Push notifications will not work. ' +
    'Generate keys with: npx web-push generate-vapid-keys'
  );
}

initializeNotificationService({
  vapidPublicKey: VAPID_PUBLIC_KEY || '',
  apiEndpoint: NOTIFICATION_API_ENDPOINT,
});

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
