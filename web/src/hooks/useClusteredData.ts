import { useState, useEffect, useCallback, useRef } from 'react';
import type { LngLatBounds } from 'maplibre-gl';
import type { Scene, Event } from '../types/scene';
import type { FeatureCollection, Point as GeoJSONPoint } from 'geojson';
import { buildGeoJSON, type FeatureProperties } from '../utils/geojson';

/**
 * Bounding box for geographic queries
 */
export interface BBox {
  north: number;
  south: number;
  east: number;
  west: number;
}

/**
 * Convert MapLibre LngLatBounds to BBox
 */
export function boundsToBox(bounds: LngLatBounds): BBox {
  return {
    north: bounds.getNorth(),
    south: bounds.getSouth(),
    east: bounds.getEast(),
    west: bounds.getWest(),
  };
}

/**
 * Options for useClusteredData hook
 */
export interface UseClusteredDataOptions {
  /**
   * API endpoint base URL
   * Default: uses VITE_API_URL env var or '/api'
   */
  apiUrl?: string;
  
  /**
   * Whether to automatically fetch on mount
   * Default: false
   */
  autoFetch?: boolean;
  
  /**
   * Debounce delay in milliseconds for bbox changes
   * Default: 300
   */
  debounceMs?: number;
}

/**
 * Result from useClusteredData hook
 */
export interface UseClusteredDataResult {
  /**
   * GeoJSON FeatureCollection for rendering
   */
  data: FeatureCollection<GeoJSONPoint, FeatureProperties>;
  
  /**
   * Whether data is currently being fetched
   */
  loading: boolean;
  
  /**
   * Error message if fetch failed
   */
  error: string | null;
  
  /**
   * Manually trigger a data fetch
   */
  refetch: () => void;
  
  /**
   * Update the bounding box and trigger fetch
   */
  updateBBox: (bbox: BBox | null) => void;
}

/**
 * Hook for fetching and clustering scene/event data based on map bounds
 * 
 * @param bbox - Initial bounding box (optional)
 * @param options - Configuration options
 * @returns Object with data, loading state, error, and refetch function
 * 
 * @example
 * const { data, loading, error, updateBBox } = useClusteredData();
 * 
 * // Update when map moves
 * map.on('moveend', () => {
 *   const bounds = map.getBounds();
 *   updateBBox(boundsToBox(bounds));
 * });
 */
export function useClusteredData(
  initialBBox: BBox | null = null,
  options: UseClusteredDataOptions = {}
): UseClusteredDataResult {
  const {
    apiUrl = import.meta.env.VITE_API_URL || '/api',
    autoFetch = false,
    debounceMs = 300,
  } = options;

  const [bbox, setBBox] = useState<BBox | null>(initialBBox);
  const [data, setData] = useState<FeatureCollection<GeoJSONPoint, FeatureProperties>>({
    type: 'FeatureCollection',
    features: [],
  });
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  
  // Use ref to track the latest fetch controller for cancellation
  const abortControllerRef = useRef<AbortController | null>(null);
  const debounceTimerRef = useRef<NodeJS.Timeout | null>(null);

  const fetchData = useCallback(async (currentBBox: BBox | null) => {
    // Cancel any pending fetch
    if (abortControllerRef.current) {
      abortControllerRef.current.abort();
    }

    if (!currentBBox) {
      setData({ type: 'FeatureCollection', features: [] });
      return;
    }

    setLoading(true);
    setError(null);

    const controller = new AbortController();
    abortControllerRef.current = controller;

    try {
      // Build query parameters
      const params = new URLSearchParams({
        north: currentBBox.north.toString(),
        south: currentBBox.south.toString(),
        east: currentBBox.east.toString(),
        west: currentBBox.west.toString(),
      });

      // Fetch scenes and events in parallel
      const [scenesRes, eventsRes] = await Promise.all([
        fetch(`${apiUrl}/scenes?${params}`, { signal: controller.signal }),
        fetch(`${apiUrl}/events?${params}`, { signal: controller.signal }),
      ]);

      if (!scenesRes.ok || !eventsRes.ok) {
        throw new Error('Failed to fetch data');
      }

      const scenes: Scene[] = await scenesRes.json();
      const events: Event[] = await eventsRes.json();

      // Build GeoJSON from entities
      const geojson = buildGeoJSON(scenes, events);
      
      setData(geojson);
      setError(null);
    } catch (err) {
      if (err instanceof Error && err.name === 'AbortError') {
        // Fetch was cancelled, ignore
        return;
      }
      
      console.error('Failed to fetch clustered data:', err);
      setError(err instanceof Error ? err.message : 'Unknown error');
    } finally {
      setLoading(false);
      abortControllerRef.current = null;
    }
  }, [apiUrl]);

  const updateBBox = useCallback((newBBox: BBox | null) => {
    setBBox(newBBox);
  }, []);

  const refetch = useCallback(() => {
    fetchData(bbox);
  }, [bbox, fetchData]);

  // Debounced fetch when bbox changes
  useEffect(() => {
    // Clear existing timer
    if (debounceTimerRef.current) {
      clearTimeout(debounceTimerRef.current);
    }

    // Debounce the fetch
    debounceTimerRef.current = setTimeout(() => {
      fetchData(bbox);
    }, debounceMs);

    return () => {
      if (debounceTimerRef.current) {
        clearTimeout(debounceTimerRef.current);
      }
    };
  }, [bbox, fetchData, debounceMs]);

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      if (abortControllerRef.current) {
        abortControllerRef.current.abort();
      }
      if (debounceTimerRef.current) {
        clearTimeout(debounceTimerRef.current);
      }
    };
  }, []);

  return {
    data,
    loading,
    error,
    refetch,
    updateBBox,
  };
}
