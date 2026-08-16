/**
 * Reconnection Slice
 * Automatic reconnection with exponential backoff for streaming sessions
 */

import type { StateCreator } from 'zustand';
import type { StreamingStore } from '../streamingStore';

/**
 * Reconnection configuration
 */
const MAX_RECONNECT_ATTEMPTS = 3;
const INITIAL_RECONNECT_DELAY = 1000; // 1 second
const MAX_RECONNECT_DELAY = 10000; // 10 seconds

/**
 * Reconnection state and actions exposed by the slice
 */
export interface ReconnectionSlice {
  reconnectAttempts: number;
  isReconnecting: boolean;
  reconnectTimeoutId: ReturnType<typeof setTimeout> | null;
  scheduleReconnect: () => void;
}

/**
 * Create the reconnection slice for composition into the streaming store
 */
export const createReconnectionSlice: StateCreator<
  StreamingStore,
  [],
  [],
  ReconnectionSlice
> = (set, get) => ({
  reconnectAttempts: 0,
  isReconnecting: false,
  reconnectTimeoutId: null,

  /**
   * Schedule reconnection with exponential backoff
   */
  scheduleReconnect: () => {
    const state = get();

    // Check if we've exceeded max attempts
    if (state.reconnectAttempts >= MAX_RECONNECT_ATTEMPTS) {
      set({
        error: 'Failed to reconnect after multiple attempts. Please try again manually.',
        isReconnecting: false,
      });
      console.error('Max reconnection attempts reached');
      return;
    }

    // Calculate backoff delay with exponential increase
    const delay = Math.min(
      INITIAL_RECONNECT_DELAY * Math.pow(2, state.reconnectAttempts),
      MAX_RECONNECT_DELAY
    );

    set({
      isReconnecting: true,
      reconnectAttempts: state.reconnectAttempts + 1,
    });

    console.info(
      `Scheduling reconnection attempt ${state.reconnectAttempts + 1} in ${delay}ms`
    );

    const timeoutId = setTimeout(() => {
      const currentState = get();

      // Only reconnect if still disconnected and have room info
      if (!currentState.isConnected && currentState.roomName) {
        console.info(
          `Attempting reconnection (attempt ${currentState.reconnectAttempts})`
        );
        currentState.connect(
          currentState.roomName,
          currentState.sceneId,
          currentState.eventId
        );
      }
    }, delay);

    set({ reconnectTimeoutId: timeoutId });
  },
});
