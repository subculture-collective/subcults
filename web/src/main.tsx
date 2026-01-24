import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import './i18n' // Initialize i18n
import App from './App.tsx'
import { initializeNotificationService } from './lib/notification-service'

// Register service worker for Web Push notifications
if ('serviceWorker' in navigator) {
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
// TODO: Replace with actual VAPID public key from backend
const VAPID_PUBLIC_KEY = import.meta.env.VITE_VAPID_PUBLIC_KEY || 'placeholder-vapid-key';
const NOTIFICATION_API_ENDPOINT = '/api/notifications/subscribe';

initializeNotificationService({
  vapidPublicKey: VAPID_PUBLIC_KEY,
  apiEndpoint: NOTIFICATION_API_ENDPOINT,
});

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
