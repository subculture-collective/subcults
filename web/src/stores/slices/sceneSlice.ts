/**
 * Scene Slice
 * Scene-specific state management with optimistic updates
 */

import { StateCreator } from 'zustand';
import { apiClient } from '../../lib/api-client';
import { Scene } from '../../types/scene';
import {
  EntityStore,
  CachedEntity,
  createFreshMetadata,
  setLoadingMetadata,
  setSuccessMetadata,
  setErrorMetadata,
  getOrCreateRequest,
} from '../entityStore';

/**
 * Create scene slice with actions
 */
export const createSceneSlice: StateCreator<
  EntityStore,
  [],
  [],
  Pick<EntityStore, 'scene' | 'fetchScene' | 'setScene' | 'markSceneStale' | 'optimisticJoinScene' | 'rollbackSceneUpdate' | 'commitSceneUpdate' | 'clearSceneError' | 'removeScene'>
> = (set, get) => ({
  scene: {
    scenes: {},
    optimisticUpdates: {},
  },

  fetchScene: async (id: string): Promise<Scene> => {
    return getOrCreateRequest(`scene:${id}`, async () => {
      const state = get();
      const cached = state.scene.scenes[id];

      // Set loading state
      set((state) => ({
        scene: {
          ...state.scene,
          scenes: {
            ...state.scene.scenes,
            [id]: {
              data: cached?.data || ({} as Scene),
              metadata: setLoadingMetadata(cached?.metadata || createFreshMetadata()),
            },
          },
        },
      }));

      try {
        // Fetch from API
        const scene = await apiClient.get<Scene>(`/scenes/${id}`);

        // Update cache with fresh data
        set((state) => ({
          scene: {
            ...state.scene,
            scenes: {
              ...state.scene.scenes,
              [id]: {
                data: scene,
                metadata: setSuccessMetadata(),
              },
            },
          },
        }));

        return scene;
      } catch (error: any) {
        const errorMessage = error?.message || 'Failed to fetch scene';

        // Update cache with error
        set((state) => ({
          scene: {
            ...state.scene,
            scenes: {
              ...state.scene.scenes,
              [id]: cached
                ? {
                    ...cached,
                    metadata: setErrorMetadata(cached.metadata, errorMessage),
                  }
                : {
                    data: {} as Scene,
                    metadata: setErrorMetadata(createFreshMetadata(), errorMessage),
                  },
            },
          },
        }));

        throw error;
      }
    });
  },

  setScene: (scene: Scene) => {
    set((state) => ({
      scene: {
        ...state.scene,
        scenes: {
          ...state.scene.scenes,
          [scene.id]: {
            data: scene,
            metadata: setSuccessMetadata(),
          },
        },
      },
    }));
  },

  markSceneStale: (id: string) => {
    set((state) => {
      const cached = state.scene.scenes[id];
      if (!cached) return state;

      return {
        scene: {
          ...state.scene,
          scenes: {
            ...state.scene.scenes,
            [id]: {
              ...cached,
              metadata: {
                ...cached.metadata,
                stale: true,
              },
            },
          },
        },
      };
    });
  },

  optimisticJoinScene: (sceneId: string, userId: string) => {
    const state = get();
    const cached = state.scene.scenes[sceneId];

    if (!cached) {
      console.warn(`Cannot optimistically join scene ${sceneId}: not in cache`);
      return;
    }

    // Save current state for rollback
    set((state) => ({
      scene: {
        ...state.scene,
        optimisticUpdates: {
          ...state.scene.optimisticUpdates,
          [sceneId]: cached.data,
        },
      },
    }));

    // Apply optimistic update (this is a placeholder - actual membership logic may vary)
    // In a real implementation, this would update membership count or status
    set((state) => ({
      scene: {
        ...state.scene,
        scenes: {
          ...state.scene.scenes,
          [sceneId]: {
            ...cached,
            data: {
              ...cached.data,
              // Optimistic update marker - actual fields depend on Scene type
              // For now, just mark as modified
            },
          },
        },
      },
    }));
  },

  rollbackSceneUpdate: (sceneId: string) => {
    const state = get();
    const backup = state.scene.optimisticUpdates[sceneId];

    if (!backup) {
      console.warn(`No optimistic update to rollback for scene ${sceneId}`);
      return;
    }

    // Restore from backup
    set((state) => {
      const cached = state.scene.scenes[sceneId];
      const { [sceneId]: removed, ...remainingUpdates } = state.scene.optimisticUpdates;

      return {
        scene: {
          ...state.scene,
          scenes: {
            ...state.scene.scenes,
            [sceneId]: {
              data: backup,
              metadata: cached?.metadata || createFreshMetadata(),
            },
          },
          optimisticUpdates: remainingUpdates,
        },
      };
    });
  },

  commitSceneUpdate: (sceneId: string) => {
    // Remove backup after successful commit
    set((state) => {
      const { [sceneId]: removed, ...remainingUpdates } = state.scene.optimisticUpdates;

      return {
        scene: {
          ...state.scene,
          optimisticUpdates: remainingUpdates,
        },
      };
    });
  },

  clearSceneError: (id: string) => {
    set((state) => {
      const cached = state.scene.scenes[id];
      if (!cached || !cached.metadata.error) return state;

      return {
        scene: {
          ...state.scene,
          scenes: {
            ...state.scene.scenes,
            [id]: {
              ...cached,
              metadata: {
                ...cached.metadata,
                error: null,
              },
            },
          },
        },
      };
    });
  },

  removeScene: (id: string) => {
    set((state) => {
      const { [id]: removed, ...remainingScenes } = state.scene.scenes;
      const { [id]: removedUpdate, ...remainingUpdates } = state.scene.optimisticUpdates;

      return {
        scene: {
          scenes: remainingScenes,
          optimisticUpdates: remainingUpdates,
        },
      };
    });
  },
});
