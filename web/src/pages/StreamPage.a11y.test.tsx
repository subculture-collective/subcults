/**
 * StreamPage Accessibility Tests
 * Validates WCAG compliance for live streaming room
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { StreamPage } from './StreamPage';
import { expectNoA11yViolations } from '../test/a11y-helpers';
import * as streamingStore from '../stores/streamingStore';

// Mock i18next
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

// Mock streaming components
vi.mock('../components/streaming', () => ({
  JoinStreamButton: vi.fn(({ onJoin, isConnecting }) => (
    <button onClick={onJoin} disabled={isConnecting}>
      {isConnecting ? 'Connecting...' : 'Join Stream'}
    </button>
  )),
  ParticipantList: vi.fn(() => <div role="list">Participants</div>),
  AudioControls: vi.fn(() => (
    <div role="group" aria-label="Audio controls">
      Controls
    </div>
  )),
  ConnectionIndicator: vi.fn(() => <div role="status">Connection Status</div>),
}));

// Mock stores
vi.mock('../stores/streamingStore', () => ({
  useStreamingStore: vi.fn((selector) => {
    const state = {
      isLocalMuted: false,
      setVolume: vi.fn(),
      toggleMute: vi.fn(),
    };
    return selector ? selector(state) : state;
  }),
  useStreamingConnection: vi.fn(() => ({
    isConnected: false,
    isConnecting: false,
    error: null,
    connectionQuality: 'good',
  })),
  useStreamingActions: vi.fn(() => ({
    connect: vi.fn(),
    disconnect: vi.fn(),
  })),
}));

vi.mock('../stores/participantStore', () => ({
  useParticipantStore: vi.fn((selector) => {
    const state = {
      getParticipantsArray: () => [],
      getLocalParticipant: () => null,
      localIdentity: null,
    };
    return selector(state);
  }),
}));

vi.mock('../stores/toastStore', () => ({
  useToasts: vi.fn(() => ({
    error: vi.fn(),
  })),
}));

describe('StreamPage - Accessibility', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('should not have any accessibility violations when disconnected', async () => {
    const { container } = render(
      <MemoryRouter initialEntries={['/stream/test-room']}>
        <Routes>
          <Route path="/stream/:room" element={<StreamPage />} />
        </Routes>
      </MemoryRouter>
    );

    await expectNoA11yViolations(container);
  });

  it('should have proper heading hierarchy', () => {
    const { container } = render(
      <MemoryRouter initialEntries={['/stream/test-room']}>
        <Routes>
          <Route path="/stream/:room" element={<StreamPage />} />
        </Routes>
      </MemoryRouter>
    );

    const h1 = container.querySelector('h1');
    expect(h1).toBeInTheDocument();
    expect(h1?.textContent).toContain('streamPage.streamRoom');
  });

  it('should use alert role for error messages', () => {
    // Mock the streaming connection to return an error
    vi.mocked(streamingStore.useStreamingConnection).mockReturnValueOnce({
      isConnected: false,
      isConnecting: false,
      error: 'Connection failed',
      connectionQuality: 'poor',
    });

    const { container } = render(
      <MemoryRouter initialEntries={['/stream/test-room']}>
        <Routes>
          <Route path="/stream/:room" element={<StreamPage />} />
        </Routes>
      </MemoryRouter>
    );

    const alert = container.querySelector('[role="alert"]');
    expect(alert).toBeInTheDocument();
    expect(alert?.textContent).toContain('Connection failed');
  });

  it('should have accessible button for joining stream', () => {
    const { getByRole } = render(
      <MemoryRouter initialEntries={['/stream/test-room']}>
        <Routes>
          <Route path="/stream/:room" element={<StreamPage />} />
        </Routes>
      </MemoryRouter>
    );

    const joinButton = getByRole('button');
    expect(joinButton).toBeInTheDocument();
    expect(joinButton).toHaveTextContent(/Join Stream/);
  });

  it('should handle missing room parameter gracefully', () => {
    const { getByText } = render(
      <MemoryRouter initialEntries={['/stream']}>
        <Routes>
          <Route path="/stream" element={<StreamPage />} />
        </Routes>
      </MemoryRouter>
    );

    expect(getByText(/streamPage.invalidRoom/)).toBeInTheDocument();
  });
});
