import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import './i18n' // Initialize i18n
import App from './App.tsx'
import { initializeNotificationService } from './lib/notification-service'
import { errorLogger } from './lib/error-logger'

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

// Register service worker for Web Push notifications (production only)
if (import.meta.env.PROD && 'serviceWorker' in navigator) {
  window.addEventListener('load', () => {
    navigator.serviceWorker
      .register('/sw.js')
      .then((registration) => {
        console.log('[ServiceWorker] Registered:', registration);
      })
      .catch((error) => {
        console.error('[ServiceWorker] Registration failed:', error);
      });
  });
}

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
