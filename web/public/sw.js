/**
 * Service Worker
 * Handles Web Push notifications and offline capabilities
 */

// Service worker version (increment on changes)
const VERSION = '1.0.0';

// Log service worker lifecycle events
self.addEventListener('install', () => {
  console.log(`[ServiceWorker v${VERSION}] Installing...`);
  // Skip waiting to activate immediately
  self.skipWaiting();
});

self.addEventListener('activate', (event) => {
  console.log(`[ServiceWorker v${VERSION}] Activating...`);
  // Claim all clients immediately
  event.waitUntil(self.clients.claim());
});

/**
 * Push Event Handler
 * Receives push notifications from the server
 */
self.addEventListener('push', (event) => {
  console.log('[ServiceWorker] Push notification received:', event);

  // Default notification data
  let title = 'Subcults';
  let options = {
    body: 'You have a new notification',
    icon: '/vite.svg',
    badge: '/vite.svg',
    vibrate: [200, 100, 200],
    tag: 'subcults-notification',
    requireInteraction: false,
  };

  // Parse notification data if available
  if (event.data) {
    try {
      const data = event.data.json();
      console.log('[ServiceWorker] Push data:', data);
      
      if (data.title) {
        title = data.title;
      }
      
      if (data.body) {
        options.body = data.body;
      }
      
      if (data.icon) {
        options.icon = data.icon;
      }
      
      if (data.tag) {
        options.tag = data.tag;
      }
      
      if (data.url) {
        options.data = { url: data.url };
      }
    } catch (error) {
      console.error('[ServiceWorker] Failed to parse push data:', error);
    }
  }

  // Show notification
  event.waitUntil(
    self.registration.showNotification(title, options)
  );
});

/**
 * Notification Click Handler
 * Handles user interaction with notifications
 */
self.addEventListener('notificationclick', (event) => {
  console.log('[ServiceWorker] Notification clicked:', event);

  // Close the notification
  event.notification.close();

  // Handle notification click action
  event.waitUntil(
    (async () => {
      const url = event.notification.data?.url || '/';
      
      // Normalize URL to absolute for comparison
      const targetUrl = new URL(url, self.location.origin).href;
      
      // Try to focus existing window/tab
      const clients = await self.clients.matchAll({
        type: 'window',
        includeUncontrolled: true,
      });

      for (const client of clients) {
        if (client.url === targetUrl && 'focus' in client) {
          return client.focus();
        }
      }

      // Open new window if no matching client found
      if (self.clients.openWindow) {
        return self.clients.openWindow(targetUrl);
      }
    })()
  );
});

/**
 * Notification Close Handler
 * Handles notification dismissal
 */
self.addEventListener('notificationclose', (event) => {
  console.log('[ServiceWorker] Notification closed:', event);
  // Future: Track notification dismissals for analytics
});

console.log(`[ServiceWorker v${VERSION}] Loaded`);
