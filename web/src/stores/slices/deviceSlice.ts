/**
 * Device Slice
 * Microphone permissions, mute state, and volume control for streaming
 */

import type { StateCreator } from 'zustand';
import {
  Room,
  RoomEvent,
  Participant as LKParticipant,
} from 'livekit-client';
import type { StreamingStore } from '../streamingStore';

/**
 * Local storage keys
 */
const VOLUME_STORAGE_KEY = 'subcults-stream-volume';
const VOLUME_DEFAULT = 100;

/**
 * Get initial volume from localStorage
 */
export function getInitialVolume(): number {
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
 * Track rooms that already have volume event handlers attached
 * to avoid registering duplicate listeners.
 */
const roomsWithVolumeHandlers = new WeakSet<Room>();

/**
 * Per-room volume getters so event handlers can always read the current volume.
 */
const roomVolumeGetters = new WeakMap<Room, () => number>();

/**
 * Apply volume to all audio tracks for a given participant.
 */
function applyVolumeToParticipant(participant: LKParticipant, normalizedVolume: number) {
  participant.audioTrackPublications.forEach((publication) => {
    if (publication.audioTrack) {
      try {
        if (typeof (publication.audioTrack as { setVolume?: unknown }).setVolume === 'function') {
          (publication.audioTrack as { setVolume: (volume: number) => void }).setVolume(
            normalizedVolume
          );
        }
      } catch (error) {
        console.warn('Volume control not supported:', error);
      }
    }
  });
}

/**
 * Apply volume to all remote audio tracks in a room
 * and ensure newly-joined participants also get the correct volume.
 *
 * @param room - The LiveKit Room instance
 * @param volume - The current volume (0-100)
 * @param getVolume - A getter function that returns the current volume from the store,
 *                    used by event handlers to always read the latest value.
 */
export function applyVolumeToRoom(
  room: Room,
  volume: number,
  getVolume?: () => number,
): void {
  const normalizedVolume = volume / 100;

  // Store volume getter for event handlers (used when new participants join)
  if (getVolume) {
    roomVolumeGetters.set(room, getVolume);
  }

  // Register event listeners once per room so that
  // participants who join later also receive the preferred volume.
  if (!roomsWithVolumeHandlers.has(room)) {
    roomsWithVolumeHandlers.add(room);

    room.on(RoomEvent.ParticipantConnected, (participant: LKParticipant) => {
      const volGetter = roomVolumeGetters.get(room);
      const effectiveVolume = volGetter ? volGetter() / 100 : normalizedVolume;
      applyVolumeToParticipant(participant, effectiveVolume);
    });

    room.on(RoomEvent.TrackSubscribed, (_track, publication, _participant: LKParticipant) => {
      if (publication.audioTrack) {
        try {
          if (typeof (publication.audioTrack as { setVolume?: unknown }).setVolume === 'function') {
            const volGetter = roomVolumeGetters.get(room);
            const effectiveVolume = volGetter ? volGetter() / 100 : normalizedVolume;
            (publication.audioTrack as { setVolume: (volume: number) => void }).setVolume(
              effectiveVolume
            );
          }
        } catch (error) {
          console.warn('Volume control not supported:', error);
        }
      }
    });
  }

  // Initial application for all current remote participants
  room.remoteParticipants.forEach((participant) => {
    applyVolumeToParticipant(participant, normalizedVolume);
  });
}

/**
 * Device state and actions exposed by the slice
 */
export interface DeviceSlice {
  volume: number;
  isLocalMuted: boolean;
  setVolume: (volume: number) => void;
  toggleMute: () => Promise<void>;
}

/**
 * Create the device slice for composition into the streaming store
 */
export const createDeviceSlice: StateCreator<
  StreamingStore,
  [],
  [],
  DeviceSlice
> = (set, get) => ({
  volume: getInitialVolume(),
  isLocalMuted: false,

  /**
   * Set playback volume (0-100)
   */
  setVolume: (volume: number) => {
    const clampedVolume = Math.max(0, Math.min(100, volume));
    persistVolume(clampedVolume);
    set({ volume: clampedVolume });

    const { room } = get();
    if (room) {
      applyVolumeToRoom(room, clampedVolume, () => get().volume);
    }
  },

  /**
   * Toggle local microphone mute
   */
  toggleMute: async () => {
    const { room } = get();
    if (!room) return;

    const targetEnabled = !room.localParticipant.isMicrophoneEnabled;

    try {
      await room.localParticipant.setMicrophoneEnabled(targetEnabled);

      const latestRoom = get().room;
      if (!latestRoom) {
        return;
      }

      const isEnabled = latestRoom.localParticipant.isMicrophoneEnabled;
      set({ isLocalMuted: !isEnabled });
    } catch (error) {
      console.error('Failed to toggle microphone mute', error);
      set({ error: 'Unable to toggle microphone' } as Partial<StreamingStore>);
    }
  },
});
