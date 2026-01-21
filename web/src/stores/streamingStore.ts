/**
 * Streaming Store
 * Manages global LiveKit connection state for persistent audio across routes
 */

import { create } from 'zustand';
import {
  Room,
  RoomEvent,
  ConnectionQuality as LKConnectionQuality,
  DisconnectReason,
} from 'livekit-client';
import { apiClient } from '../lib/api-client';
import type { ConnectionQuality } from '../types/streaming';

/**
 * Reconnection configuration
 */
const MAX_RECONNECT_ATTEMPTS = 3;
const INITIAL_RECONNECT_DELAY = 1000; // 1 second
const MAX_RECONNECT_DELAY = 10000; // 10 seconds

/**
 * Local storage keys
 */
const VOLUME_STORAGE_KEY = 'subcults-stream-volume';
const VOLUME_DEFAULT = 100;

/**
 * Streaming state
 */
interface StreamingState {
  // Connection state
  room: Room | null;
  roomName: string | null;
  isConnected: boolean;
  isConnecting: boolean;
  error: string | null;
  connectionQuality: ConnectionQuality;
  
  // Audio state
  volume: number;
  isMuted: boolean;
  
  // Reconnection state
  reconnectAttempts: number;
  isReconnecting: boolean;
  
  // Room metadata
  sceneId?: string;
  eventId?: string;
}

/**
 * Streaming actions
 */
interface StreamingActions {
  // Connection management
  connect: (roomName: string, sceneId?: string, eventId?: string) => Promise<void>;
  disconnect: () => void;
  
  // Audio controls
  setVolume: (volume: number) => void;
  toggleMute: () => Promise<void>;
  
  // Internal state management
  setConnectionQuality: (quality: ConnectionQuality) => void;
  setError: (error: string | null) => void;
  
  // Reconnection
  scheduleReconnect: () => void;
  
  // Initialization
  initialize: () => void;
}

export type StreamingStore = StreamingState & StreamingActions;

/**
 * Get initial volume from localStorage
 */
function getInitialVolume(): number {
  try {
    const stored = localStorage.getItem(VOLUME_STORAGE_KEY);
    if (stored) {
      const parsed = parseInt(stored, 10);
      if (!isNaN(parsed) && parsed >= 0 && parsed <= 100) {
        return parsed;
      }
    }
  } catch (error) {
    console.warn('Failed to read volume from localStorage:', error);
  }
  return VOLUME_DEFAULT;
}

/**
 * Persist volume to localStorage
 */
function persistVolume(volume: number): void {
  try {
    localStorage.setItem(VOLUME_STORAGE_KEY, volume.toString());
  } catch (error) {
    console.warn('Failed to persist volume to localStorage:', error);
  }
}

/**
 * Map LiveKit connection quality to our quality type
 */
function mapConnectionQuality(lkQuality: LKConnectionQuality): ConnectionQuality {
  switch (lkQuality) {
    case LKConnectionQuality.Excellent:
      return 'excellent';
    case LKConnectionQuality.Good:
      return 'good';
    case LKConnectionQuality.Poor:
      return 'poor';
    default:
      return 'unknown';
  }
}

/**
 * Streaming store with global connection management
 */
export const useStreamingStore = create<StreamingStore>((set, get) => ({
  // Initial state
  room: null,
  roomName: null,
  isConnected: false,
  isConnecting: false,
  error: null,
  connectionQuality: 'unknown',
  volume: getInitialVolume(),
  isMuted: false,
  reconnectAttempts: 0,
  isReconnecting: false,

  /**
   * Initialize the store (call on app startup)
   */
  initialize: () => {
    // Load persisted volume
    const volume = getInitialVolume();
    set({ volume });
  },

  /**
   * Connect to a LiveKit room
   */
  connect: async (roomName: string, sceneId?: string, eventId?: string) => {
    const state = get();
    
    // Don't reconnect if already connected to same room
    if (state.isConnected && state.roomName === roomName) {
      return;
    }
    
    // Disconnect from any existing connection
    if (state.room) {
      state.disconnect();
    }
    
    set({ 
      isConnecting: true, 
      error: null,
      roomName,
      sceneId,
      eventId,
      reconnectAttempts: 0,
    });

    try {
      // Fetch token
      const { token } = await apiClient.getLiveKitToken(
        roomName,
        sceneId,
        eventId
      );

      // Create room
      const room = new Room();
      
      // Set up event listeners
      room.on(RoomEvent.ConnectionQualityChanged, (quality: LKConnectionQuality) => {
        get().setConnectionQuality(mapConnectionQuality(quality));
      });

      room.on(RoomEvent.Disconnected, (reason?: DisconnectReason) => {
        const state = get();
        const isClientInitiated = reason === DisconnectReason.CLIENT_INITIATED;
        
        if (!isClientInitiated) {
          // Unexpected disconnect - attempt reconnection
          console.warn('Unexpected disconnect:', reason);
          set({ 
            isConnected: false,
            error: 'Connection lost. Attempting to reconnect...',
          });
          state.scheduleReconnect();
        } else {
          // Clean disconnect
          set({
            isConnected: false,
            isConnecting: false,
            room: null,
            error: null,
          });
        }
      });

      // Connect to room
      const wsUrl = import.meta.env.VITE_LIVEKIT_WS_URL;
      if (!wsUrl || typeof wsUrl !== 'string' || wsUrl.trim() === '') {
        throw new Error('LiveKit WebSocket URL is not configured');
      }
      
      await room.connect(wsUrl, token);

      // Enable local microphone
      await room.localParticipant.setMicrophoneEnabled(true);

      // Apply current volume setting to room
      const { volume } = get();
      applyVolumeToRoom(room, volume);

      set({
        room,
        isConnected: true,
        isConnecting: false,
        error: null,
        reconnectAttempts: 0,
        isReconnecting: false,
      });

      console.info('Connected to room:', roomName);
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : 'Failed to connect to room';
      console.error('Connection error:', errorMessage);
      
      set({
        isConnecting: false,
        error: errorMessage,
      });

      // Schedule reconnection on failure
      get().scheduleReconnect();
    }
  },

  /**
   * Disconnect from current room
   */
  disconnect: () => {
    const { room } = get();
    
    if (room) {
      room.removeAllListeners();
      room.disconnect();
    }

    set({
      room: null,
      roomName: null,
      isConnected: false,
      isConnecting: false,
      error: null,
      connectionQuality: 'unknown',
      reconnectAttempts: 0,
      isReconnecting: false,
    });

    console.info('Disconnected from room');
  },

  /**
   * Set playback volume (0-100)
   */
  setVolume: (volume: number) => {
    // Clamp volume
    const clampedVolume = Math.max(0, Math.min(100, volume));
    
    // Persist to localStorage
    persistVolume(clampedVolume);
    
    // Update state
    set({ volume: clampedVolume });
    
    // Apply to current room
    const { room } = get();
    if (room) {
      applyVolumeToRoom(room, clampedVolume);
    }
  },

  /**
   * Toggle local microphone mute
   */
  toggleMute: async () => {
    const { room } = get();
    if (!room) return;

    const isEnabled = room.localParticipant.isMicrophoneEnabled;
    await room.localParticipant.setMicrophoneEnabled(!isEnabled);
    
    set({ isMuted: !isEnabled });
  },

  /**
   * Set connection quality
   */
  setConnectionQuality: (quality: ConnectionQuality) => {
    set({ connectionQuality: quality });
  },

  /**
   * Set error message
   */
  setError: (error: string | null) => {
    set({ error });
  },

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

    console.info(`Scheduling reconnection attempt ${state.reconnectAttempts + 1} in ${delay}ms`);

    setTimeout(() => {
      const currentState = get();
      
      // Only reconnect if still disconnected and have room info
      if (!currentState.isConnected && currentState.roomName) {
        console.info(`Attempting reconnection (attempt ${currentState.reconnectAttempts})`);
        currentState.connect(
          currentState.roomName,
          currentState.sceneId,
          currentState.eventId
        );
      }
    }, delay);
  },
}));

/**
 * Apply volume to all remote audio tracks in a room
 */
function applyVolumeToRoom(room: Room, volume: number): void {
  const normalizedVolume = volume / 100;

  room.remoteParticipants.forEach((participant) => {
    participant.audioTrackPublications.forEach((publication) => {
      if (publication.audioTrack) {
        try {
          // Type guard for setVolume method
          if (
            typeof (publication.audioTrack as { setVolume?: unknown }).setVolume === 'function'
          ) {
            (publication.audioTrack as { setVolume: (volume: number) => void }).setVolume(normalizedVolume);
          }
        } catch (error) {
          console.warn('Volume control not supported:', error);
        }
      }
    });
  });
}

/**
 * Hook for streaming connection state (optimized selectors)
 */
export function useStreamingConnection() {
  return useStreamingStore((state) => ({
    isConnected: state.isConnected,
    isConnecting: state.isConnecting,
    roomName: state.roomName,
    error: state.error,
    connectionQuality: state.connectionQuality,
  }));
}

/**
 * Hook for streaming audio controls (optimized selectors)
 */
export function useStreamingAudio() {
  return useStreamingStore((state) => ({
    volume: state.volume,
    isMuted: state.isMuted,
    setVolume: state.setVolume,
    toggleMute: state.toggleMute,
  }));
}

/**
 * Hook for streaming actions (stable references)
 */
export function useStreamingActions() {
  return useStreamingStore((state) => ({
    connect: state.connect,
    disconnect: state.disconnect,
  }));
}
