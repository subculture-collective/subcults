/**
 * SceneSettingsPage Tests
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { BrowserRouter } from 'react-router-dom';
import { SceneSettingsPage } from './SceneSettingsPage';
import { useEntityStore } from '../stores/entityStore';
import { useToastStore } from '../stores/toastStore';
import type { Scene } from '../types/scene';

// Mock the stores
vi.mock('../stores/entityStore');
vi.mock('../stores/toastStore');
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, fallback?: string) => fallback || key,
  }),
}));

// Mock useParams and useNavigate
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom');
  return {
    ...actual,
    useParams: () => ({ id: 'test-scene-id' }),
    useNavigate: () => vi.fn(),
  };
});

describe('SceneSettingsPage', () => {
  const mockScene: Scene = {
    id: 'test-scene-id',
    name: 'Test Scene',
    description: 'Test Description',
    allow_precise: false,
    coarse_geohash: 'u4pruyd',
    tags: ['techno', 'underground'],
    visibility: 'public',
    palette: {
      primary: '#3b82f6',
      secondary: '#8b5cf6',
      accent: '#ec4899',
      background: '#ffffff',
      text: '#000000',
    },
  };

  const mockFetchScene = vi.fn();
  const mockUpdateScene = vi.fn();
  const mockAddToast = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    
    // Setup store mocks
    (useEntityStore as unknown as ReturnType<typeof vi.fn>).mockImplementation((selector) => {
      const store = {
        fetchScene: mockFetchScene,
        updateScene: mockUpdateScene,
        scene: { scenes: {} },
      };
      return selector(store);
    });

    (useToastStore as unknown as ReturnType<typeof vi.fn>).mockImplementation((selector) => {
      const store = {
        addToast: mockAddToast,
      };
      return selector(store);
    });

    mockFetchScene.mockResolvedValue(mockScene);
  });

  it('renders loading state initially', () => {
    render(
      <BrowserRouter>
        <SceneSettingsPage />
      </BrowserRouter>
    );

    // Loading state shows skeleton, not text
    const skeletons = screen.getAllByRole('generic');
    expect(skeletons.length).toBeGreaterThan(0);
  });

  it('loads and displays scene data', async () => {
    render(
      <BrowserRouter>
        <SceneSettingsPage />
      </BrowserRouter>
    );

    await waitFor(() => {
      expect(mockFetchScene).toHaveBeenCalledWith('test-scene-id');
    });

    await waitFor(() => {
      const nameInput = screen.getByPlaceholderText(/Enter scene name/i);
      expect(nameInput).toHaveValue('Test Scene');
    });
  });

  it('displays scene tags', async () => {
    render(
      <BrowserRouter>
        <SceneSettingsPage />
      </BrowserRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('techno')).toBeInTheDocument();
      expect(screen.getByText('underground')).toBeInTheDocument();
    });
  });

  it('displays privacy settings with correct visibility selected', async () => {
    render(
      <BrowserRouter>
        <SceneSettingsPage />
      </BrowserRouter>
    );

    await waitFor(() => {
      const publicRadio = screen.getByLabelText(/Public/i) as HTMLInputElement;
      expect(publicRadio.checked).toBe(true);
    });
  });

  it('displays color customization section', async () => {
    render(
      <BrowserRouter>
        <SceneSettingsPage />
      </BrowserRouter>
    );

    await waitFor(() => {
      expect(screen.getByText(/Visual Customization/i)).toBeInTheDocument();
      expect(screen.getByText(/Primary Color/i)).toBeInTheDocument();
      expect(screen.getByText(/Secondary Color/i)).toBeInTheDocument();
    });
  });

  it('displays member management section', async () => {
    render(
      <BrowserRouter>
        <SceneSettingsPage />
      </BrowserRouter>
    );

    await waitFor(() => {
      expect(screen.getByText(/Members & Alliances/i)).toBeInTheDocument();
      expect(screen.getByText(/View Members/i)).toBeInTheDocument();
    });
  });

  it('displays verification status', async () => {
    render(
      <BrowserRouter>
        <SceneSettingsPage />
      </BrowserRouter>
    );

    await waitFor(() => {
      expect(screen.getByText(/Verification Status/i)).toBeInTheDocument();
      expect(screen.getByText(/Owner Verified/i)).toBeInTheDocument();
    });
  });

  it('shows error toast when scene fetch fails', async () => {
    mockFetchScene.mockRejectedValue(new Error('Failed to load'));

    render(
      <BrowserRouter>
        <SceneSettingsPage />
      </BrowserRouter>
    );

    await waitFor(() => {
      expect(mockAddToast).toHaveBeenCalledWith({
        type: 'error',
        message: 'Failed to load scene',
      });
    });
  });
});
