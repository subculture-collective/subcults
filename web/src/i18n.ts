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
