/**
 * JoinStreamButton Component
 * Button to initiate joining a LiveKit audio room
 */

import React from 'react';
import { useTranslation } from 'react-i18next';

export interface JoinStreamButtonProps {
  isConnected: boolean;
  isConnecting: boolean;
  onJoin: () => void;
  disabled?: boolean;
}

export const JoinStreamButton: React.FC<JoinStreamButtonProps> = ({
  isConnected,
  isConnecting,
  onJoin,
  disabled = false,
}) => {
  const { t } = useTranslation();
  
  const buttonText = isConnecting
    ? t('streaming.joinButton.connecting')
    : isConnected
    ? t('streaming.joinButton.connected')
    : t('streaming.joinButton.join');

  return (
    <button
      onClick={onJoin}
      disabled={disabled || isConnected || isConnecting}
      className={`join-stream-button ${isConnected ? 'connected' : ''} ${
        isConnecting ? 'connecting' : ''
      }`}
      aria-label={buttonText}
      style={{
        padding: '0.75rem 1.5rem',
        fontSize: '1rem',
        fontWeight: 600,
        borderRadius: '0.5rem',
        border: 'none',
        cursor: disabled || isConnected || isConnecting ? 'not-allowed' : 'pointer',
        backgroundColor: isConnected
          ? '#10b981'
          : isConnecting
          ? '#6b7280'
          : '#3b82f6',
        color: 'white',
        opacity: disabled ? 0.5 : 1,
        transition: 'all 0.2s ease',
        minWidth: '150px',
      }}
      onMouseEnter={(e) => {
        if (!disabled && !isConnected && !isConnecting) {
          e.currentTarget.style.backgroundColor = '#2563eb';
        }
      }}
      onMouseLeave={(e) => {
        if (!disabled && !isConnected && !isConnecting) {
          e.currentTarget.style.backgroundColor = '#3b82f6';
        }
      }}
    >
      {buttonText}
    </button>
  );
};
