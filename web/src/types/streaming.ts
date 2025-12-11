/**
 * Streaming Types
 * Types for LiveKit audio streaming functionality
 */

/**
 * LiveKit token response from backend
 */
export interface LiveKitTokenResponse {
  token: string;
  expires_at: string; // RFC3339 timestamp
}

/**
 * Request payload for LiveKit token
 */
export interface LiveKitTokenRequest {
  room_id: string;
  scene_id?: string;
  event_id?: string;
}

/**
 * Participant in an audio room
 */
export interface Participant {
  identity: string;
  name?: string;
  isLocal: boolean;
  isMuted: boolean;
  isSpeaking: boolean;
}

/**
 * Connection quality levels
 */
export type ConnectionQuality = 'excellent' | 'good' | 'poor' | 'unknown';

/**
 * Audio room state
 */
export interface AudioRoomState {
  roomName: string;
  isConnected: boolean;
  isConnecting: boolean;
  participants: Participant[];
  localParticipant: Participant | null;
  connectionQuality: ConnectionQuality;
  error: string | null;
}

/**
 * Stats for determining connection quality
 */
export interface ConnectionStats {
  packetLoss: number; // 0-1 percentage
  jitter: number; // milliseconds
  latency: number; // milliseconds
}
