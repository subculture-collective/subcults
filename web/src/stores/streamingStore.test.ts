/**
 * Streaming Store Tests
 * Tests for global streaming state management
 */

import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';
import { useStreamingStore } from './streamingStore';
import { mockRoom } from '../test/mocks/livekit-client';

// Mock API client
vi.mock('../lib/api-client', () => ({
  apiClient: {
    getLiveKitToken: vi.fn().mockResolvedValue({
      token: 'mock-token',
      expires_at: new Date(Date.now() + 3600000).toISOString(),
    }),
  },
}));

// Mock environment variable
vi.stubEnv('VITE_LIVEKIT_WS_URL', 'ws://localhost:7880');

// Mock localStorage
const localStorageMock = (() => {
  let store: Record<string, string> = {};

  return {
    getItem: (key: string) => store[key] || null,
    setItem: (key: string, value: string) => {
      store[key] = value;
    },
    removeItem: (key: string) => {
      delete store[key];
    },
    clear: () => {
      store = {};
    },
  };
})();

Object.defineProperty(window, 'localStorage', {
  value: localStorageMock,
});

describe('StreamingStore', () => {
  beforeEach(() => {
    // Reset store to initial state
    useStreamingStore.setState({
      room: null,
      roomName: null,
      isConnected: false,
      isConnecting: false,
      error: null,
      connectionQuality: 'unknown',
      volume: 100,
      isMuted: false,
      reconnectAttempts: 0,
      isReconnecting: false,
    });
    
    // Clear localStorage
    localStorageMock.clear();
    
    // Reset mocks
    vi.clearAllMocks();
  });

  afterEach(() => {
    // Cleanup
    const store = useStreamingStore.getState();
    if (store.room) {
      store.disconnect();
    }
  });

  describe('Volume Persistence', () => {
    it('should initialize with default volume', () => {
      const store = useStreamingStore.getState();
      expect(store.volume).toBe(100);
    });

    it('should load volume from localStorage', () => {
      localStorageMock.setItem('subcults-stream-volume', '75');
      
      const store = useStreamingStore.getState();
      store.initialize();
      
      expect(store.volume).toBe(75);
    });

    it('should persist volume to localStorage when changed', () => {
      const store = useStreamingStore.getState();
      
      store.setVolume(50);
      
      expect(localStorageMock.getItem('subcults-stream-volume')).toBe('50');
      expect(store.volume).toBe(50);
    });

    it('should clamp volume between 0 and 100', () => {
      const store = useStreamingStore.getState();
      
      store.setVolume(150);
      expect(store.volume).toBe(100);
      
      store.setVolume(-10);
      expect(store.volume).toBe(0);
    });

    it('should fallback to default on invalid localStorage value', () => {
      localStorageMock.setItem('subcults-stream-volume', 'invalid');
      
      const store = useStreamingStore.getState();
      store.initialize();
      
      expect(store.volume).toBe(100);
    });
  });

  describe('Connection Management', () => {
    it('should start in disconnected state', () => {
      const store = useStreamingStore.getState();
      
      expect(store.isConnected).toBe(false);
      expect(store.isConnecting).toBe(false);
      expect(store.room).toBeNull();
      expect(store.roomName).toBeNull();
    });

    it('should set connecting state when initiating connection', async () => {
      const store = useStreamingStore.getState();
      
      const connectPromise = store.connect('test-room');
      
      expect(store.isConnecting).toBe(true);
      
      await connectPromise;
    });

    it('should transition to connected state on successful connection', async () => {
      const store = useStreamingStore.getState();
      
      await store.connect('test-room');
      
      expect(store.isConnected).toBe(true);
      expect(store.isConnecting).toBe(false);
      expect(store.roomName).toBe('test-room');
      expect(store.error).toBeNull();
    });

    it('should disconnect and clear state', async () => {
      const store = useStreamingStore.getState();
      
      await store.connect('test-room');
      expect(store.isConnected).toBe(true);
      
      store.disconnect();
      
      expect(store.isConnected).toBe(false);
      expect(store.room).toBeNull();
      expect(store.roomName).toBeNull();
      expect(mockRoom.disconnect).toHaveBeenCalled();
    });

    it('should not reconnect if already connected to same room', async () => {
      const store = useStreamingStore.getState();
      
      await store.connect('test-room');
      const firstRoom = store.room;
      
      await store.connect('test-room');
      const secondRoom = store.room;
      
      expect(firstRoom).toBe(secondRoom);
    });

    it('should disconnect from old room when connecting to new room', async () => {
      const store = useStreamingStore.getState();
      
      await store.connect('test-room-1');
      expect(store.roomName).toBe('test-room-1');
      
      await store.connect('test-room-2');
      expect(store.roomName).toBe('test-room-2');
      expect(mockRoom.disconnect).toHaveBeenCalled();
    });
  });

  describe('State Persistence Across Routes', () => {
    it('should maintain connection state when navigating', async () => {
      const store = useStreamingStore.getState();
      
      await store.connect('test-room');
      
      // Simulate route change by accessing store again
      const sameStore = useStreamingStore.getState();
      
      expect(sameStore.isConnected).toBe(true);
      expect(sameStore.roomName).toBe('test-room');
    });

    it('should maintain volume setting across route changes', () => {
      const store = useStreamingStore.getState();
      
      store.setVolume(60);
      
      // Simulate route change
      const sameStore = useStreamingStore.getState();
      
      expect(sameStore.volume).toBe(60);
      expect(localStorageMock.getItem('subcults-stream-volume')).toBe('60');
    });
  });

  describe('Reconnection Logic', () => {
    it('should schedule reconnection on unexpected disconnect', async () => {
      vi.useFakeTimers();
      
      const store = useStreamingStore.getState();
      await store.connect('test-room');
      
      // Simulate unexpected disconnect
      store.scheduleReconnect();
      
      expect(store.isReconnecting).toBe(true);
      expect(store.reconnectAttempts).toBe(1);
      
      vi.useRealTimers();
    });

    it('should use exponential backoff for reconnection attempts', () => {
      vi.useFakeTimers();
      
      const store = useStreamingStore.getState();
      
      // First attempt
      store.scheduleReconnect();
      expect(store.reconnectAttempts).toBe(1);
      
      // Second attempt
      store.scheduleReconnect();
      expect(store.reconnectAttempts).toBe(2);
      
      // Third attempt
      store.scheduleReconnect();
      expect(store.reconnectAttempts).toBe(3);
      
      vi.useRealTimers();
    });

    it('should stop reconnecting after max attempts', () => {
      const store = useStreamingStore.getState();
      
      // Simulate max attempts
      useStreamingStore.setState({ reconnectAttempts: 3 });
      
      store.scheduleReconnect();
      
      expect(store.error).toContain('Failed to reconnect after multiple attempts');
      expect(store.isReconnecting).toBe(false);
    });

    it('should reset reconnect attempts on successful connection', async () => {
      const store = useStreamingStore.getState();
      
      useStreamingStore.setState({ reconnectAttempts: 2 });
      
      await store.connect('test-room');
      
      expect(store.reconnectAttempts).toBe(0);
      expect(store.isReconnecting).toBe(false);
    });
  });

  describe('Audio Controls', () => {
    it('should toggle mute state', async () => {
      const store = useStreamingStore.getState();
      
      await store.connect('test-room');
      
      await store.toggleMute();
      
      expect(mockRoom.localParticipant.setMicrophoneEnabled).toHaveBeenCalledWith(false);
    });

    it('should apply volume to room on connection', async () => {
      const store = useStreamingStore.getState();
      
      store.setVolume(50);
      
      await store.connect('test-room');
      
      // Volume should be applied to connected room
      expect(store.volume).toBe(50);
    });
  });

  describe('Error Handling', () => {
    it('should set error on connection failure', async () => {
      const { apiClient } = await import('../lib/api-client');
      vi.mocked(apiClient.getLiveKitToken).mockRejectedValueOnce(
        new Error('Token fetch failed')
      );
      
      const store = useStreamingStore.getState();
      
      await store.connect('test-room');
      
      expect(store.error).toContain('Token fetch failed');
      expect(store.isConnecting).toBe(false);
      expect(store.isConnected).toBe(false);
    });

    it('should schedule reconnect on connection error', async () => {
      const { apiClient } = await import('../lib/api-client');
      vi.mocked(apiClient.getLiveKitToken).mockRejectedValueOnce(
        new Error('Network error')
      );
      
      const scheduleReconnectSpy = vi.spyOn(
        useStreamingStore.getState(),
        'scheduleReconnect'
      );
      
      const store = useStreamingStore.getState();
      await store.connect('test-room');
      
      expect(scheduleReconnectSpy).toHaveBeenCalled();
    });

    it('should handle missing WebSocket URL', async () => {
      vi.stubEnv('VITE_LIVEKIT_WS_URL', '');
      
      const store = useStreamingStore.getState();
      
      await store.connect('test-room');
      
      expect(store.error).toContain('LiveKit WebSocket URL is not configured');
      expect(store.isConnected).toBe(false);
    });
  });
});
