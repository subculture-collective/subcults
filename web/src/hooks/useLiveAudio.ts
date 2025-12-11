/**
 * useLiveAudio Hook
 * Manages LiveKit audio room connection, participants, and token refresh
 */

import { useState, useEffect, useCallback, useRef } from 'react';
import {
  Room,
  RoomEvent,
  ConnectionQuality as LKConnectionQuality,
  Participant as LKParticipant,
  Track,
  DisconnectReason,
} from 'livekit-client';
import { apiClient } from '../lib/api-client';
import type {
  AudioRoomState,
  Participant,
  ConnectionQuality,
} from '../types/streaming';

/**
 * Hook options
 */
export interface UseLiveAudioOptions {
  sceneId?: string;
  eventId?: string;
  onError?: (error: Error) => void;
}

/**
 * Hook result
 */
export interface UseLiveAudioResult extends AudioRoomState {
  connect: () => Promise<void>;
  disconnect: () => void;
  toggleMute: () => Promise<void>;
  setVolume: (volume: number) => void;
}

/**
 * Token expiry threshold for refresh (30 seconds before expiry)
 */
const TOKEN_REFRESH_THRESHOLD_MS = 30 * 1000;

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
function convertParticipant(
  participant: LKParticipant,
  isLocal: boolean
): Participant {
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
 * useLiveAudio hook
 * Manages LiveKit room connection lifecycle and state
 */
export function useLiveAudio(
  roomName: string | null,
  options: UseLiveAudioOptions = {}
): UseLiveAudioResult {
  const [state, setState] = useState<AudioRoomState>({
    roomName: roomName || '',
    isConnected: false,
    isConnecting: false,
    participants: [],
    localParticipant: null,
    connectionQuality: 'unknown',
    error: null,
  });

  const roomRef = useRef<Room | null>(null);
  const tokenExpiryRef = useRef<number | null>(null);
  const refreshTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  /**
   * Update participants list from room
   */
  const updateParticipants = useCallback(() => {
    const room = roomRef.current;
    if (!room) return;

    const remoteParticipants = Array.from(room.remoteParticipants.values()).map(
      (p) => convertParticipant(p, false)
    );

    const localPart = room.localParticipant
      ? convertParticipant(room.localParticipant, true)
      : null;

    setState((prev) => ({
      ...prev,
      participants: remoteParticipants,
      localParticipant: localPart,
    }));
  }, []);

  /**
   * Schedule token refresh before expiry
   */
  const scheduleTokenRefresh = useCallback(
    async (expiresAt: string) => {
      // Clear any existing refresh timeout
      if (refreshTimeoutRef.current) {
        clearTimeout(refreshTimeoutRef.current);
        refreshTimeoutRef.current = null;
      }

      const expiryTime = new Date(expiresAt).getTime();
      tokenExpiryRef.current = expiryTime;

      const now = Date.now();
      const timeUntilRefresh = expiryTime - now - TOKEN_REFRESH_THRESHOLD_MS;

      // Only schedule if we have time before expiry
      if (timeUntilRefresh > 0) {
        refreshTimeoutRef.current = setTimeout(async () => {
          const room = roomRef.current;
          if (!room || !roomName) return;

          try {
            // Fetch new token
            const { expires_at } = await apiClient.getLiveKitToken(
              roomName,
              options.sceneId,
              options.eventId
            );

            // Note: Token refresh in LiveKit 2.x requires reconnection
            // For now, we'll let the connection expire and require manual rejoin
            // TODO: Implement seamless reconnection with new token
            console.log('Token refresh scheduled for:', expires_at);

            // Schedule next refresh
            scheduleTokenRefresh(expires_at);
          } catch (error) {
            console.error('Failed to refresh LiveKit token:', error);
            // On refresh failure, let the connection drop naturally
            // The user can manually reconnect if needed
          }
        }, timeUntilRefresh);
      }
    },
    [roomName, options.sceneId, options.eventId]
  );

  /**
   * Connect to room
   */
  const connect = useCallback(async () => {
    if (!roomName || state.isConnected || state.isConnecting) return;

    setState((prev) => ({ ...prev, isConnecting: true, error: null }));

    try {
      // Fetch token
      const { token, expires_at } = await apiClient.getLiveKitToken(
        roomName,
        options.sceneId,
        options.eventId
      );

      // Create and connect room
      const room = new Room();
      roomRef.current = room;

      // Set up event listeners before connecting
      room.on(RoomEvent.ParticipantConnected, updateParticipants);
      room.on(RoomEvent.ParticipantDisconnected, updateParticipants);
      room.on(RoomEvent.LocalTrackPublished, updateParticipants);
      room.on(RoomEvent.LocalTrackUnpublished, updateParticipants);
      room.on(RoomEvent.TrackMuted, updateParticipants);
      room.on(RoomEvent.TrackUnmuted, updateParticipants);
      room.on(RoomEvent.ActiveSpeakersChanged, updateParticipants);

      // Connection quality monitoring
      room.on(RoomEvent.ConnectionQualityChanged, (quality: LKConnectionQuality) => {
        setState((prev) => ({
          ...prev,
          connectionQuality: mapConnectionQuality(quality),
        }));
      });

      // Error handling
      room.on(RoomEvent.Disconnected, (reason?: DisconnectReason) => {
        const reasonStr = reason ? String(reason) : undefined;
        setState((prev) => ({
          ...prev,
          isConnected: false,
          isConnecting: false,
          error: reasonStr || null,
        }));
      });

      // Connect to room
      const wsUrl = import.meta.env.VITE_LIVEKIT_WS_URL || 'wss://subcults.livekit.cloud';
      await room.connect(wsUrl, token);

      // Enable local microphone
      await room.localParticipant.setMicrophoneEnabled(true);

      // Update state
      setState((prev) => ({
        ...prev,
        isConnected: true,
        isConnecting: false,
        roomName,
      }));

      // Update participants
      updateParticipants();

      // Schedule token refresh
      scheduleTokenRefresh(expires_at);
    } catch (error) {
      const errorMessage =
        error instanceof Error ? error.message : 'Failed to connect to room';
      
      setState((prev) => ({
        ...prev,
        isConnecting: false,
        error: errorMessage,
      }));

      if (options.onError) {
        options.onError(
          error instanceof Error ? error : new Error(errorMessage)
        );
      }

      // Clean up on error
      if (roomRef.current) {
        roomRef.current.disconnect();
        roomRef.current = null;
      }
    }
  }, [
    roomName,
    state.isConnected,
    state.isConnecting,
    options.sceneId,
    options.eventId,
    options.onError,
    updateParticipants,
    scheduleTokenRefresh,
  ]);

  /**
   * Disconnect from room
   */
  const disconnect = useCallback(() => {
    if (refreshTimeoutRef.current) {
      clearTimeout(refreshTimeoutRef.current);
      refreshTimeoutRef.current = null;
    }

    if (roomRef.current) {
      roomRef.current.disconnect();
      roomRef.current = null;
    }

    setState({
      roomName: roomName || '',
      isConnected: false,
      isConnecting: false,
      participants: [],
      localParticipant: null,
      connectionQuality: 'unknown',
      error: null,
    });
  }, [roomName]);

  /**
   * Toggle local microphone mute
   */
  const toggleMute = useCallback(async () => {
    const room = roomRef.current;
    if (!room) return;

    const isMuted = room.localParticipant.isMicrophoneEnabled;
    await room.localParticipant.setMicrophoneEnabled(!isMuted);
    updateParticipants();
  }, [updateParticipants]);

  /**
   * Set playback volume (0-100)
   * Note: This adjusts the volume of remote audio tracks
   */
  const setVolume = useCallback((volume: number) => {
    const room = roomRef.current;
    if (!room) return;

    // Clamp volume between 0 and 100
    const clampedVolume = Math.max(0, Math.min(100, volume));
    const normalizedVolume = clampedVolume / 100;

    // Set volume for all remote audio tracks
    room.remoteParticipants.forEach((participant) => {
      participant.audioTrackPublications.forEach((publication) => {
        if (publication.audioTrack) {
          // Volume control may not be available in all LiveKit versions
          // This is a best-effort approach
          try {
            (publication.audioTrack as any).setVolume?.(normalizedVolume);
          } catch (error) {
            console.warn('Volume control not supported:', error);
          }
        }
      });
    });
  }, []);

  /**
   * Cleanup on unmount
   */
  useEffect(() => {
    return () => {
      disconnect();
    };
  }, [disconnect]);

  return {
    ...state,
    connect,
    disconnect,
    toggleMute,
    setVolume,
  };
}
