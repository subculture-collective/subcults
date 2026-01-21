/**
 * i18n Configuration
 * Internationalization setup using i18next
 */

import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';

// Translation resources
const resources = {
  en: {
    translation: {
      streaming: {
        joinButton: {
          join: 'Join Stream',
          connecting: 'Connecting...',
          connected: 'Connected',
        },
        participantList: {
          empty: 'No participants in the room',
          you: 'You',
          speaking: 'Speaking...',
        },
        audioControls: {
          mute: 'Mute microphone',
          unmute: 'Unmute microphone',
          volumeControl: 'Volume control',
          volumeLabel: 'Volume',
          leave: 'Leave',
          leaveRoom: 'Leave room',
        },
        connectionIndicator: {
          quality: 'Connection quality',
          excellent: 'Excellent',
          good: 'Good',
          poor: 'Poor',
          unknown: 'Unknown',
        },
        streamPage: {
          invalidRoom: 'Invalid Room',
          noRoomId: 'No room ID provided',
          streamRoom: 'Stream Room',
          room: 'Room',
          error: 'Error',
          participants: 'Participants',
        },
        miniPlayer: {
          label: 'Mini player',
          nowPlaying: 'Now Playing',
          mute: 'Mute',
          unmute: 'Unmute',
          volumeControl: 'Volume control',
          volume: 'Volume',
          volumeSlider: 'Volume slider',
          leave: 'Leave',
          quality: {
            excellent: 'Excellent connection',
            good: 'Good connection',
            poor: 'Poor connection',
            unknown: 'Unknown connection',
          },
        },
      },
    },
  },
};

i18n
  .use(initReactI18next)
  .init({
    resources,
    lng: 'en',
    fallbackLng: 'en',
    interpolation: {
      escapeValue: false,
    },
  });

export default i18n;
