/**
 * useEvents Hook
 * Hook for accessing multiple events with derived selectors
 */

import { useMemo } from 'react';
import { useEntityStore } from '../stores/entityStore';
import { Event } from '../types/scene';

export interface UseEventsOptions {
  filterByScene?: string;
  sortBy?: 'name' | 'date';
  includeLoading?: boolean;
}

export interface UseEventsResult {
  events: Event[];
  upcomingCount: number;
  loading: boolean;
}

/**
 * Hook to access and filter events from cache
 * Provides derived data like upcoming events count
 */
export function useEvents(options: UseEventsOptions = {}): UseEventsResult {
  const { filterByScene, sortBy = 'name', includeLoading = false } = options;

  const cachedEvents = useEntityStore((state) => state.event.events);

  // Memoized filtered and sorted events
  const events = useMemo(() => {
    const eventList = Object.values(cachedEvents)
      .filter((cached) => {
        // Skip loading entries unless explicitly requested
        if (!includeLoading && cached.metadata.loading) return false;
        
        // Skip entries with errors or no data
        if (cached.metadata.error || !cached.data.id) return false;

        return true;
      })
      .map((cached) => cached.data);

    // Apply filters
    let filtered = eventList;

    if (filterByScene) {
      filtered = filtered.filter((event) => event.scene_id === filterByScene);
    }

    // Sort events
    const sorted = [...filtered].sort((a, b) => {
      if (sortBy === 'date') {
        // Sort by date (this requires the Event type to have a date field)
        // For now, sort by name as fallback
        return a.name.localeCompare(b.name);
      }
      return a.name.localeCompare(b.name);
    });

    return sorted;
  }, [cachedEvents, filterByScene, sortBy, includeLoading]);

  // Calculate upcoming events count (placeholder - needs date logic)
  const upcomingCount = useMemo(() => {
    // In a real implementation, this would filter by future dates
    return events.length;
  }, [events]);

  // Check if any event is loading
  const loading = useMemo(() => {
    return Object.values(cachedEvents).some((cached) => cached.metadata.loading);
  }, [cachedEvents]);

  return {
    events,
    upcomingCount,
    loading,
  };
}

/**
 * Hook to get events for a specific scene
 */
export function useSceneEvents(sceneId: string | undefined): UseEventsResult {
  return useEvents({ filterByScene: sceneId });
}

/**
 * Hook to get upcoming events sorted by date
 */
export function useUpcomingEvents(): UseEventsResult {
  return useEvents({ sortBy: 'date' });
}
