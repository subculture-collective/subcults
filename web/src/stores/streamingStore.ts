/**
 * Streaming Store
 * Manages global LiveKit connection state for persistent audio across routes.
 *
 * Composed from focused slices:
 * - deviceSlice: microphone permissions, mute state, volume control
 * - reconnectionSlice: exponential backoff reconnection logic
 *
 * Core responsibility: LiveKit connection lifecycle (connect, disconnect, token exchange)
 */

import { create } from 'zustand';
import {
  Room,
  RoomEvent,
  ConnectionQuality as LKConnectionQuality,
  DisconnectReason,
  Participant as LKParticipant,
  Track,
} from 'livekit-client';
import { apiClient } from '../lib/api-client';
import { useParticipantStore, normalizeIdentity } from '../stores/participantStore';
import { useLatencyStore } from '../stores/latencyStore';
import { createDeviceSlice } from './slices/deviceSlice';
import { createReconnectionSlice } from './slices/reconnectionSlice';
import { getInitialVolume, applyVolumeToRoom } from './slices/deviceSlice';
import type { DeviceSlice } from './slices/deviceSlice';
import type { ReconnectionSlice } from './slices/reconnectionSlice';
import type { ConnectionQuality, Participant } from '../types/streaming';

// ---------------------------------------------------------------------------
// Connection state (core responsibility of this store)
// ---------------------------------------------------------------------------

interface ConnectionState {
  room: Room | null;
  roomName: string | null;
  isConnected: boolean;
  isConnecting: boolean;
  error: string | null;
  connectionQuality: ConnectionQuality;
  sceneId?: string;
  eventId?: string;
}

interface ConnectionActions {
  connect: (roomName: string, sceneId?: string, eventId?: string) => Promise<void>;
  disconnect: () => void;
  setConnectionQuality: (quality: ConnectionQuality) => void;
  setError: (error: string | null) => void;
  initialize: () => void;
}

// ---------------------------------------------------------------------------
// Combined store type — used by slices' StateCreator<StreamingStore>
// ---------------------------------------------------------------------------

export type StreamingStore = ConnectionState &
  ConnectionActions &
  DeviceSlice &
  ReconnectionSlice;

// ---------------------------------------------------------------------------
// Helpers (connection-specific)
// ---------------------------------------------------------------------------

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
 * Convert LiveKit participant to our Participant type
 */
function convertParticipant(participant: LKParticipant, isLocal: boolean): Participant {
  const audioTrack = participant.getTrackPublication(Track.Source.Microphone);

  return {
    identity: participant.identity,
    name: participant.name || participant.identity,
    isLocal,
    isMuted: audioTrack?.isMuted ?? true,
    isSpeaking: participant.isSpeaking,
  };
}

/**
 * Update participants list from room
 */
function updateParticipants(room: Room) {
  const store = useParticipantStore.getState();

  // Update remote participants in store
  room.remoteParticipants.forEach((participant) => {
    const converted = convertParticipant(participant, false);
    store.addParticipant(converted);
  });

  // Update local participant in store
  if (room.localParticipant) {
    const localPart = convertParticipant(room.localParticipant, true);
    store.addParticipant(localPart);
    store.setLocalIdentity(localPart.identity);
  }
}

// ---------------------------------------------------------------------------
// Connection actions (factory — accepts set/get from create)
// ---------------------------------------------------------------------------

function createConnectionActions(
  set: (partial: Partial<StreamingStore>) => void,
  get: () => StreamingStore,
): ConnectionState & ConnectionActions {
  return {
    room: null,
    roomName: null,
    isConnected: false,
    isConnecting: false,
    error: null,
    connectionQuality: 'unknown',
    sceneId: undefined,
    eventId: undefined,

    /**
     * Initialize the store (call on app startup)
     * Re-reads volume from localStorage in case it was set after store creation
     */
    initialize: () => {
      const storedVolume = getInitialVolume();
      set({ volume: storedVolume });
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

      // Reset latency tracking for new join attempt
      const latencyStore = useLatencyStore.getState();
      latencyStore.resetLatency();
      useLatencyStore.setState((prev) => ({
        ...prev,
        lastLatency: null,
      }));

      set({
        isConnecting: true,
        error: null,
        roomName,
        sceneId,
        eventId,
        reconnectAttempts: 0,
      });

      try {
        // Fetch token (t1: token received)
        const { token } = await apiClient.getLiveKitToken(roomName, sceneId, eventId);

        // Record token received timestamp
        latencyStore.recordTokenReceived();

        // Create room
        const room = new Room();

        // Set up connection quality monitoring
        room.on(RoomEvent.ConnectionQualityChanged, (quality: LKConnectionQuality) => {
          get().setConnectionQuality(mapConnectionQuality(quality));
        });

        // Set up disconnect handler
        room.on(RoomEvent.Disconnected, (reason?: DisconnectReason) => {
          const currentState = get();
          const isClientInitiated = reason === DisconnectReason.CLIENT_INITIATED;

          if (!isClientInitiated) {
            // Unexpected disconnect - attempt reconnection
            console.warn('Unexpected disconnect:', reason);
            set({
              isConnected: false,
              error: 'Connection lost. Attempting to reconnect...',
            });
            currentState.scheduleReconnect();
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

        // Set up participant tracking event listeners
        room.on(RoomEvent.ParticipantConnected, (participant: LKParticipant) => {
          const participantStore = useParticipantStore.getState();
          const converted = convertParticipant(participant, false);
          participantStore.addParticipant(converted);
          updateParticipants(room);
        });

        room.on(RoomEvent.ParticipantDisconnected, (participant: LKParticipant) => {
          const participantStore = useParticipantStore.getState();
          participantStore.removeParticipant(participant.identity);
          updateParticipants(room);
        });

        room.on(RoomEvent.LocalTrackPublished, () => {
          updateParticipants(room);
        });

        room.on(RoomEvent.LocalTrackUnpublished, () => {
          updateParticipants(room);
        });

        room.on(RoomEvent.TrackMuted, (publication, participant: LKParticipant) => {
          if (publication.source === Track.Source.Microphone) {
            const participantStore = useParticipantStore.getState();
            participantStore.updateParticipantMute(participant.identity, true);
            updateParticipants(room);
          }
        });

        room.on(RoomEvent.TrackUnmuted, (publication, participant: LKParticipant) => {
          if (publication.source === Track.Source.Microphone) {
            const participantStore = useParticipantStore.getState();
            participantStore.updateParticipantMute(participant.identity, false);
            updateParticipants(room);
          }
        });

        room.on(RoomEvent.ActiveSpeakersChanged, (speakers: LKParticipant[]) => {
          const participantStore = useParticipantStore.getState();
          const allParticipants = participantStore.getParticipantsArray();
          const speakerIdentities = new Set(speakers.map((s) => normalizeIdentity(s.identity)));

          allParticipants.forEach((p) => {
            const shouldBeSpeaking = speakerIdentities.has(p.identity);
            if (p.isSpeaking !== shouldBeSpeaking) {
              participantStore.updateParticipantSpeaking(p.identity, shouldBeSpeaking);
            }
          });

          updateParticipants(room);
        });

        // Track first audio subscription for latency measurement (t3)
        let firstAudioTracked = false;
        room.on(RoomEvent.TrackSubscribed, (track) => {
          if (!firstAudioTracked && track.kind === 'audio') {
            const latencyStore = useLatencyStore.getState();
            latencyStore.recordFirstAudioSubscribed();
            latencyStore.finalizeLatency();
            firstAudioTracked = true;
          }
        });

        // Connect to room
        const wsUrl = import.meta.env.VITE_LIVEKIT_WS_URL;
        if (!wsUrl || typeof wsUrl !== 'string' || wsUrl.trim() === '') {
          throw new Error('LiveKit WebSocket URL is not configured');
        }

        await room.connect(wsUrl, token);

        // Record room connected timestamp (t2)
        latencyStore.recordRoomConnected();

        // Enable local microphone
        await room.localParticipant.setMicrophoneEnabled(true);

        // Update participants
        updateParticipants(room);

        // Apply current volume setting to room (passing the volume getter for event handlers)
        const { volume } = get();
        applyVolumeToRoom(room, volume, () => get().volume);

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
        const errorMessage =
          error instanceof Error ? error.message : 'Failed to connect to room';
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
      const { room, reconnectTimeoutId } = get();

      if (room) {
        room.removeAllListeners();
        room.disconnect();
      }

      // Clear any pending reconnect timeout
      if (reconnectTimeoutId) {
        clearTimeout(reconnectTimeoutId);
      }

      // Clear participant store
      const participantStore = useParticipantStore.getState();
      participantStore.clearParticipants();

      set({
        room: null,
        roomName: null,
        isConnected: false,
        isConnecting: false,
        error: null,
        connectionQuality: 'unknown',
        reconnectAttempts: 0,
        isReconnecting: false,
        reconnectTimeoutId: null,
      });

      console.info('Disconnected from room');
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
  };
}

// ---------------------------------------------------------------------------
// Composed store
// ---------------------------------------------------------------------------

export const useStreamingStore = create<StreamingStore>((set, get, store) => {
  const connection = createConnectionActions(set, get);

  return {
    ...createDeviceSlice(set, get, store),
    ...createReconnectionSlice(set, get, store),
    ...connection,
  };
});

// ---------------------------------------------------------------------------
// Individual streaming state selectors - use primitives to avoid infinite loops
// ---------------------------------------------------------------------------

export const useStreamingIsConnected = () => useStreamingStore((state) => state.isConnected);
export const useStreamingIsConnecting = () => useStreamingStore((state) => state.isConnecting);
export const useStreamingRoomName = () => useStreamingStore((state) => state.roomName);
export const useStreamingError = () => useStreamingStore((state) => state.error);
export const useStreamingConnectionQuality = () =>
  useStreamingStore((state) => state.connectionQuality);
export const useStreamingVolume = () => useStreamingStore((state) => state.volume);
export const useStreamingIsLocalMuted = () => useStreamingStore((state) => state.isLocalMuted);
export const useStreamingSetVolume = () => useStreamingStore((state) => state.setVolume);
export const useStreamingToggleMute = () => useStreamingStore((state) => state.toggleMute);
export const useStreamingConnect = () => useStreamingStore((state) => state.connect);
export const useStreamingDisconnect = () => useStreamingStore((state) => state.disconnect);

// ---------------------------------------------------------------------------
// Deprecated convenience hooks — kept for backwards compatibility
// ---------------------------------------------------------------------------

/**
 * @deprecated Use individual selectors instead (useStreamingIsConnected, etc.)
 * Kept for backwards compatibility - will cause re-renders on any state change
 */
export function useStreamingConnection() {
  const isConnected = useStreamingIsConnected();
  const isConnecting = useStreamingIsConnecting();
  const roomName = useStreamingRoomName();
  const error = useStreamingError();
  const connectionQuality = useStreamingConnectionQuality();
  return { isConnected, isConnecting, roomName, error, connectionQuality };
}

/**
 * @deprecated Use individual selectors instead (useStreamingVolume, etc.)
 */
export function useStreamingAudio() {
  const volume = useStreamingVolume();
  const isLocalMuted = useStreamingIsLocalMuted();
  const setVolume = useStreamingSetVolume();
  const toggleMute = useStreamingToggleMute();
  return { volume, isLocalMuted, setVolume, toggleMute };
}

/**
 * @deprecated Use individual selectors instead (useStreamingConnect, etc.)
 */
export function useStreamingActions() {
  const connect = useStreamingConnect();
  const disconnect = useStreamingDisconnect();
  return { connect, disconnect };
}
