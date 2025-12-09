/**
 * useEvent Hook
 * Hook for fetching and subscribing to an event with automatic stale revalidation
 */

import { useEffect, useMemo } from 'react';
import { useEntityStore } from '../stores/entityStore';
import { isStale, TTL_CONFIG } from '../stores/entityStore';
import { Event } from '../types/scene';

export interface UseEventResult {
  event: Event | null;
  loading: boolean;
  error: string | null;
  refetch: () => Promise<void>;
}

/**
 * Hook to fetch and subscribe to an event
 * Automatically fetches if missing or stale (stale-while-revalidate)
 */
export function useEvent(id: string | undefined): UseEventResult {
  const cached = useEntityStore((state) => (id ? state.event.events[id] : undefined));
  const fetchEvent = useEntityStore((state) => state.fetchEvent);
  const markEventStale = useEntityStore((state) => state.markEventStale);

  // Fetch event if missing or stale
  useEffect(() => {
    if (!id) return;

    const shouldFetch = !cached || isStale(cached.metadata, TTL_CONFIG.DEFAULT);

    if (shouldFetch && !cached?.metadata.loading) {
      // Background fetch if stale (stale-while-revalidate)
      fetchEvent(id).catch((error) => {
        console.error(`Failed to fetch event ${id}:`, error);
      });
    }
  }, [id, cached, fetchEvent]);

  // Memoized refetch function
  const refetch = useMemo(
    () => async () => {
      if (!id) return;
      // Mark as stale to force refetch
      markEventStale(id);
      await fetchEvent(id);
    },
    [id, fetchEvent, markEventStale]
  );

  return {
    event: cached?.data || null,
    loading: cached?.metadata.loading || false,
    error: cached?.metadata.error || null,
    refetch,
  };
}
