/**
 * NotificationSettings Component
 * User interface for managing Web Push notification preferences
 */

import React, { useState, useEffect } from 'react';
import { useNotificationStore } from '../stores/notificationStore';
import {
  isBrowserSupported,
  requestNotificationPermission,
  subscribeToPushNotifications,
  unsubscribeFromPushNotifications,
  sendSubscriptionToBackend,
  deleteSubscriptionFromBackend,
  getCurrentSubscription,
} from '../lib/notification-service';

export const NotificationSettings: React.FC = () => {
  const {
    permission,
    isSubscribed,
    subscription,
    isLoading,
    error,
    setPermission,
    setSubscription,
    setLoading,
    setError,
  } = useNotificationStore();

  const [browserSupported] = useState(isBrowserSupported());

  // Sync permission state on mount
  useEffect(() => {
    if (browserSupported && typeof Notification !== 'undefined') {
      setPermission(Notification.permission);
    }
  }, [browserSupported, setPermission]);

  // Check for existing subscription on mount
  useEffect(() => {
    const checkExistingSubscription = async () => {
      if (!browserSupported) return;

      try {
        const currentSub = await getCurrentSubscription();
        if (currentSub && !subscription) {
          setSubscription(currentSub);
        }
      } catch (error) {
        console.error('[NotificationSettings] Failed to check existing subscription:', error);
      }
    };

    checkExistingSubscription();
  }, [browserSupported, subscription, setSubscription]);

  const handleEnableNotifications = async () => {
    setLoading(true);
    setError(null);

    try {
      // Step 1: Request permission
      const newPermission = await requestNotificationPermission();
      setPermission(newPermission);

      if (newPermission !== 'granted') {
        setError('Notification permission was denied. Please enable notifications in your browser settings.');
        return;
      }

      // Step 2: Subscribe to push notifications
      const subscriptionData = await subscribeToPushNotifications();

      // Step 3: Send subscription to backend
      await sendSubscriptionToBackend(subscriptionData);

      // Step 4: Update state
      setSubscription(subscriptionData);

      // TODO: Increment metrics for successful subscription
      console.log('[NotificationSettings] Successfully subscribed to notifications');
    } catch (error) {
      console.error('[NotificationSettings] Failed to enable notifications:', error);
      setError(error instanceof Error ? error.message : 'Failed to enable notifications');
    } finally {
      setLoading(false);
    }
  };

  const handleDisableNotifications = async () => {
    setLoading(true);
    setError(null);

    try {
      // Step 1: Delete subscription from backend
      if (subscription) {
        await deleteSubscriptionFromBackend(subscription);
      }

      // Step 2: Unsubscribe from push notifications
      await unsubscribeFromPushNotifications();

      // Step 3: Update state
      setSubscription(null);

      console.log('[NotificationSettings] Successfully unsubscribed from notifications');
    } catch (error) {
      console.error('[NotificationSettings] Failed to disable notifications:', error);
      setError(error instanceof Error ? error.message : 'Failed to disable notifications');
    } finally {
      setLoading(false);
    }
  };

  if (!browserSupported) {
    return (
      <div className="bg-background-secondary border border-border rounded-lg p-6 theme-transition">
        <h2 className="text-2xl font-semibold mb-4 text-foreground">Notifications</h2>
        <div className="bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg p-4">
          <p className="text-yellow-800 dark:text-yellow-200">
            <strong>Browser not supported:</strong> Your browser does not support Web Push notifications.
            Please use a modern browser like Chrome, Firefox, Edge, or Safari to enable notifications.
          </p>
        </div>
        <div className="mt-4 text-sm text-foreground-muted">
          <p className="mb-2">
            <strong>Privacy Note:</strong> Notifications are completely optional and require your explicit consent.
          </p>
          <p>
            We respect your privacy and will only send notifications about events that matter to you,
            such as new events near your location, live stream starts, or membership approvals.
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="bg-background-secondary border border-border rounded-lg p-6 theme-transition">
      <h2 className="text-2xl font-semibold mb-4 text-foreground">Notifications</h2>

      <div className="space-y-4">
        {/* Status Display */}
        <div className="flex items-center justify-between py-4 border-b border-border">
          <div>
            <h3 className="text-lg font-medium text-foreground mb-1">Push Notifications</h3>
            <p className="text-sm text-foreground-secondary">
              {isSubscribed
                ? 'You will receive notifications about new events, streams, and updates'
                : 'Enable notifications to stay updated on events and streams'}
            </p>
          </div>

          <div className="flex items-center gap-4">
            {/* Status Badge */}
            <span
              className={`px-3 py-1 rounded-full text-sm font-medium ${
                isSubscribed
                  ? 'bg-green-100 dark:bg-green-900/30 text-green-800 dark:text-green-200'
                  : 'bg-gray-100 dark:bg-gray-800 text-gray-800 dark:text-gray-200'
              }`}
            >
              {isSubscribed ? 'Enabled' : 'Disabled'}
            </span>

            {/* Toggle Button */}
            <button
              onClick={isSubscribed ? handleDisableNotifications : handleEnableNotifications}
              disabled={isLoading || permission === 'denied'}
              className={`px-4 py-2 rounded-lg font-medium transition-colors disabled:opacity-50 disabled:cursor-not-allowed ${
                isSubscribed
                  ? 'bg-red-600 hover:bg-red-700 text-white'
                  : 'bg-brand-primary hover:bg-brand-primary-dark text-white'
              }`}
            >
              {isLoading ? 'Processing...' : isSubscribed ? 'Disable' : 'Enable'}
            </button>
          </div>
        </div>

        {/* Error Message */}
        {error && (
          <div className="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg p-4">
            <p className="text-red-800 dark:text-red-200 text-sm">
              <strong>Error:</strong> {error}
            </p>
          </div>
        )}

        {/* Permission Denied Message */}
        {permission === 'denied' && (
          <div className="bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg p-4">
            <p className="text-yellow-800 dark:text-yellow-200 text-sm">
              <strong>Permission denied:</strong> You have blocked notifications for this site.
              To enable notifications, please update your browser settings and reload the page.
            </p>
          </div>
        )}

        {/* Subscription Details (for debugging) */}
        {subscription && (
          <details className="text-sm">
            <summary className="cursor-pointer text-foreground-secondary hover:text-foreground">
              Subscription Details
            </summary>
            <pre className="mt-2 p-3 bg-background rounded border border-border overflow-x-auto text-xs text-foreground-muted">
              {JSON.stringify({ endpoint: subscription.endpoint }, null, 2)}
            </pre>
          </details>
        )}

        {/* Privacy Information */}
        <div className="mt-6 pt-6 border-t border-border">
          <h3 className="text-lg font-medium text-foreground mb-3">Privacy &amp; Consent</h3>
          <div className="text-sm text-foreground-secondary space-y-2">
            <p>
              <strong>Explicit Opt-In:</strong> Notifications are completely optional and will only
              be enabled if you explicitly grant permission.
            </p>
            <p>
              <strong>What We'll Notify You About:</strong> New events near you, live stream starts,
              scene membership approvals, and other updates you've subscribed to.
            </p>
            <p>
              <strong>Your Control:</strong> You can disable notifications at any time by clicking
              the "Disable" button above. You can also manage notification permissions in your
              browser settings.
            </p>
            <p>
              <strong>Privacy First:</strong> We respect your privacy. Notification subscriptions
              are stored securely and are only used to send you relevant updates. We never sell or
              share your subscription data with third parties.
            </p>
          </div>
        </div>
      </div>
    </div>
  );
};
