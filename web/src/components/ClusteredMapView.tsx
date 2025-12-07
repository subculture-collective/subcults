import { useEffect, useRef } from 'react';
import { useClusteredData, boundsToBox } from '../hooks/useClusteredData';
import { MapView, type MapViewHandle, type MapViewProps } from './MapView';
import type { Map } from 'maplibre-gl';

/**
 * Props for ClusteredMapView component
 */
export interface ClusteredMapViewProps extends Omit<MapViewProps, 'onLoad'> {
  /**
   * Custom handler when map loads (receives map instance)
   */
  onLoad?: (map: Map) => void;
}

/**
 * ClusteredMapView - Enhanced MapView with scene/event clustering
 * 
 * This component wraps MapView and adds:
 * - Automatic data fetching based on map bounds
 * - Real-time cluster rendering for scenes and events
 * - Click handlers for cluster expansion
 * - Separate icon styling for scenes vs events
 * 
 * Privacy considerations:
 * - Respects location consent flags from backend
 * - Uses coarse geohash coordinates when precise location is not allowed
 */
export function ClusteredMapView(props: ClusteredMapViewProps) {
  const mapRef = useRef<MapViewHandle>(null);
  const { data, updateBBox, loading, error } = useClusteredData(null, { debounceMs: 300 });
  const mapInstanceRef = useRef<Map | null>(null);

  // Handle map load event
  const handleMapLoad = (map: Map) => {
    mapInstanceRef.current = map;

    // Remove placeholder source and layers
    if (map.getLayer('clusters-placeholder')) {
      map.removeLayer('clusters-placeholder');
    }
    if (map.getSource('scenes-placeholder')) {
      map.removeSource('scenes-placeholder');
    }

    // Add real cluster source
    map.addSource('scenes-events', {
      type: 'geojson',
      data,
      cluster: true,
      clusterMaxZoom: 14,
      clusterRadius: 50,
    });

    // Add cluster circle layer with size buckets
    map.addLayer({
      id: 'clusters',
      type: 'circle',
      source: 'scenes-events',
      filter: ['has', 'point_count'],
      paint: {
        // Size based on point_count
        'circle-color': [
          'step',
          ['get', 'point_count'],
          '#51bbd6', // < 10 points
          10,
          '#f1f075', // 10-100 points
          100,
          '#f28cb1', // 100+ points
        ],
        'circle-radius': [
          'step',
          ['get', 'point_count'],
          20, // < 10 points
          10,
          30, // 10-100 points
          100,
          40, // 100+ points
        ],
      },
    });

    // Add cluster count labels
    map.addLayer({
      id: 'cluster-count',
      type: 'symbol',
      source: 'scenes-events',
      filter: ['has', 'point_count'],
      layout: {
        'text-field': '{point_count_abbreviated}',
        'text-font': ['DIN Offc Pro Medium', 'Arial Unicode MS Bold'],
        'text-size': 12,
      },
    });

    // Add unclustered scene points layer
    map.addLayer({
      id: 'unclustered-scene-point',
      type: 'circle',
      source: 'scenes-events',
      filter: ['all', ['!', ['has', 'point_count']], ['==', ['get', 'type'], 'scene']],
      paint: {
        'circle-color': '#11b4da',
        'circle-radius': 8,
        'circle-stroke-width': 2,
        'circle-stroke-color': '#fff',
      },
    });

    // Add unclustered event points layer
    map.addLayer({
      id: 'unclustered-event-point',
      type: 'circle',
      source: 'scenes-events',
      filter: ['all', ['!', ['has', 'point_count']], ['==', ['get', 'type'], 'event']],
      paint: {
        'circle-color': '#f28cb1',
        'circle-radius': 6,
        'circle-stroke-width': 2,
        'circle-stroke-color': '#fff',
      },
    });

    // Add click handler for clusters to expand/zoom
    map.on('click', 'clusters', (e) => {
      const features = map.queryRenderedFeatures(e.point, {
        layers: ['clusters'],
      });
      
      if (!features.length) return;

      const clusterId = features[0].properties?.cluster_id;
      const source = map.getSource('scenes-events');
      
      if (source && 'getClusterExpansionZoom' in source && clusterId !== undefined) {
        source.getClusterExpansionZoom(clusterId, (err, zoom) => {
          if (err) {
            console.error('Failed to expand cluster:', err);
            return;
          }
          if (!features[0].geometry || features[0].geometry.type !== 'Point') return;

          map.easeTo({
            center: features[0].geometry.coordinates as [number, number],
            zoom: zoom !== null ? zoom : map.getZoom() + 2,
          });
        });
      }
    });

    // Change cursor on hover over clusters
    map.on('mouseenter', 'clusters', () => {
      map.getCanvas().style.cursor = 'pointer';
    });
    map.on('mouseleave', 'clusters', () => {
      map.getCanvas().style.cursor = '';
    });

    // Add cursor for unclustered points
    map.on('mouseenter', 'unclustered-scene-point', () => {
      map.getCanvas().style.cursor = 'pointer';
    });
    map.on('mouseleave', 'unclustered-scene-point', () => {
      map.getCanvas().style.cursor = '';
    });
    map.on('mouseenter', 'unclustered-event-point', () => {
      map.getCanvas().style.cursor = 'pointer';
    });
    map.on('mouseleave', 'unclustered-event-point', () => {
      map.getCanvas().style.cursor = '';
    });

    // Update bbox on map move
    map.on('moveend', () => {
      const bounds = map.getBounds();
      updateBBox(boundsToBox(bounds));
    });

    // Initial bbox fetch
    const bounds = map.getBounds();
    updateBBox(boundsToBox(bounds));

    // Call custom onLoad handler if provided
    if (props.onLoad) {
      props.onLoad(map);
    }
  };

  // Update source data when clustered data changes
  useEffect(() => {
    const map = mapInstanceRef.current;
    if (!map) return;

    const source = map.getSource('scenes-events');
    if (source && 'setData' in source) {
      source.setData(data);
    }
  }, [data]);

  // Display loading/error states
  if (error) {
    console.error('Clustering error:', error);
  }
  if (loading) {
    console.log('Loading clustered data...');
  }

  return <MapView {...props} ref={mapRef} onLoad={handleMapLoad} />;
}
