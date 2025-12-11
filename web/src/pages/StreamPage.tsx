/**
 * StreamPage Component
 * Live audio streaming room
 * This is lazy-loaded due to heavy dependencies
 */

import React, { useMemo } from 'react';
import { useParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { useLiveAudio } from '../hooks/useLiveAudio';
import {
  JoinStreamButton,
  ParticipantList,
  AudioControls,
  ConnectionIndicator,
} from '../components/streaming';
import { useToasts } from '../stores/toastStore';

export const StreamPage: React.FC = () => {
  const { room } = useParams<{ room: string }>();
  const { t } = useTranslation();
  const { error: showError } = useToasts();

  const {
    isConnected,
    isConnecting,
    participants,
    localParticipant,
    connectionQuality,
    error,
    connect,
    disconnect,
    toggleMute,
    setVolume,
  } = useLiveAudio(room || null, {
    onError: (err) => {
      showError(err.message || t('streaming.streamPage.error'));
    },
  });

  // Calculate total participant count
  const participantCount = useMemo(
    () => participants.length + (localParticipant ? 1 : 0),
    [participants.length, localParticipant]
  );

  if (!room) {
    return (
      <div style={{ padding: '2rem', textAlign: 'center' }}>
        <h1>{t('streaming.streamPage.invalidRoom')}</h1>
        <p>{t('streaming.streamPage.noRoomId')}</p>
      </div>
    );
  }

  return (
    <div
      style={{
        padding: '2rem',
        maxWidth: '800px',
        margin: '0 auto',
        color: 'white',
      }}
    >
      <div style={{ marginBottom: '2rem' }}>
        <h1 style={{ fontSize: '2rem', fontWeight: 700, marginBottom: '0.5rem' }}>
          {t('streaming.streamPage.streamRoom')}
        </h1>
        <p style={{ color: '#9ca3af' }}>
          {t('streaming.streamPage.room')}: {room}
        </p>
      </div>

      {/* Error Display */}
      {error && (
        <div
          style={{
            padding: '1rem',
            marginBottom: '1.5rem',
            backgroundColor: '#7f1d1d',
            borderRadius: '0.5rem',
            border: '1px solid #ef4444',
          }}
          role="alert"
        >
          <strong>{t('streaming.streamPage.error')}:</strong> {error}
        </div>
      )}

      {/* Join Button */}
      {!isConnected && (
        <div style={{ marginBottom: '2rem' }}>
          <JoinStreamButton
            isConnected={isConnected}
            isConnecting={isConnecting}
            onJoin={connect}
          />
        </div>
      )}

      {/* Connected View */}
      {isConnected && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '1.5rem' }}>
          {/* Connection Quality */}
          <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
            <ConnectionIndicator quality={connectionQuality} />
          </div>

          {/* Audio Controls */}
          <AudioControls
            isMuted={localParticipant?.isMuted ?? true}
            onToggleMute={toggleMute}
            onLeave={disconnect}
            onVolumeChange={setVolume}
          />

          {/* Participants */}
          <div>
            <h2
              style={{
                fontSize: '1.25rem',
                fontWeight: 600,
                marginBottom: '1rem',
              }}
            >
              {t('streaming.streamPage.participants')} ({participantCount})
            </h2>
            <ParticipantList
              participants={participants}
              localParticipant={localParticipant}
            />
          </div>
        </div>
      )}
    </div>
  );
};

