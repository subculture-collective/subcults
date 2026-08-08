import { useEffect, useMemo } from 'react';
import type { GeoJSONSource, Map as MapLibreMap } from 'maplibre-gl';
import type { AppearanceSummary } from '../../types/touring';

export interface TourMapLayerProps {
  map: MapLibreMap | null;
  appearances: AppearanceSummary[];
  onSelectEvent?: (eventID: string) => void;
}

const SOURCE_ID = 'tour-appearances';
const LAYER_ID = 'tour-appearance-points';

/** Groups appearances by event, because an Event is the map's atomic occurrence. */
export function groupAppearancesByEvent(appearances: AppearanceSummary[]): Map<string, AppearanceSummary[]> {
  return appearances.reduce((groups, appearance) => {
    const group = groups.get(appearance.event.id) ?? [];
    group.push(appearance);
    groups.set(appearance.event.id, group);
    return groups;
  }, new Map<string, AppearanceSummary[]>());
}

export function TourMapLayer({ map, appearances, onSelectEvent }: TourMapLayerProps) {
  const groups = useMemo(() => groupAppearancesByEvent(appearances), [appearances]);
  const data = useMemo<GeoJSON.FeatureCollection>(() => ({
    type: 'FeatureCollection',
    features: Array.from(groups.entries()).flatMap(([eventID, group]) => {
      const point = group[0].event.occurrence.display_point;
      if (!point) return [];
      return [{
        type: 'Feature' as const,
        geometry: { type: 'Point' as const, coordinates: [point.lng, point.lat] },
        properties: { event_id: eventID, appearance_count: group.length, title: group[0].event.title },
      }];
    }),
  }), [groups]);

  useEffect(() => {
    if (!map) return;
    const source = map.getSource(SOURCE_ID) as GeoJSONSource | undefined;
    if (source) source.setData(data);
    else {
      map.addSource(SOURCE_ID, { type: 'geojson', data });
      map.addLayer({ id: LAYER_ID, type: 'circle', source: SOURCE_ID, paint: {
        'circle-color': '#bd93f9', 'circle-radius': 8, 'circle-stroke-color': '#0d0d14', 'circle-stroke-width': 2,
      } });
    }
    const select = (event: { features?: Array<{ properties?: { event_id?: string } }> }) => {
      const id = event.features?.[0]?.properties?.event_id;
      if (id) onSelectEvent?.(id);
    };
    map.on('click', LAYER_ID, select);
    return () => { map.off('click', LAYER_ID, select); };
  }, [map, data, onSelectEvent]);

  return null;
}
