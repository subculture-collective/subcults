import { useEffect, useState } from 'react';
import type { Map as MapLibreMap } from 'maplibre-gl';
import { useParams } from 'react-router-dom';
import { AppearanceCard } from '../components/discovery/AppearanceCard';
import { TourMapLayer } from '../components/discovery/TourMapLayer';
import { MapView } from '../components/MapView';
import type { TouringDetailResponse } from '../types/touring';

function readDetail(response: unknown): TouringDetailResponse {
  const value = response as TouringDetailResponse & { data?: TouringDetailResponse };
  return value.data ?? value;
}

export function TourDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [detail, setDetail] = useState<TouringDetailResponse | null>(null);
  const [map, setMap] = useState<MapLibreMap | null>(null);
  const [selectedEventID, setSelectedEventID] = useState<string | null>(null);
  const [error, setError] = useState(false);
  useEffect(() => {
    if (!id) return;
    let active = true;
    fetch(`${import.meta.env.VITE_API_URL || '/api'}/tours/${id}`)
      .then(async (response) => response.ok ? readDetail(await response.json()) : Promise.reject(response))
      .then((result) => { if (active) setDetail(result); })
      .catch(() => { if (active) setError(true); });
    return () => { active = false; };
  }, [id]);
  if (error) return <main className="p-8"><h1>Tour unavailable</h1><p>Touring details could not be loaded.</p></main>;
  if (!detail) return <main className="p-8" aria-busy="true"><h1>Loading tour…</h1></main>;
  const selectedAppearances = selectedEventID
    ? detail.appearances.filter((appearance) => appearance.event.id === selectedEventID)
    : [];
  return <main className="mx-auto max-w-5xl p-6 md:p-8">
    <p className="font-mono text-xs uppercase tracking-wide text-neon-cyan">Tour</p>
    <h1 className="mt-1">{detail.tour?.title ?? 'Tour'}</h1>
    <section aria-label="Tour occurrence map" className="mt-6 h-72 border border-border">
      <MapView className="h-full" onLoad={setMap} />
      <TourMapLayer map={map} appearances={detail.appearances} onSelectEvent={setSelectedEventID} />
    </section>
    {selectedAppearances.length > 0 && <section className="mt-4 border border-border bg-background-secondary p-4" aria-live="polite" aria-labelledby="selected-event-appearances">
      <h2 id="selected-event-appearances" className="m-0 text-lg">{selectedAppearances[0].event.title}</h2>
      <p className="mt-1 text-sm text-foreground-secondary">{selectedAppearances.length} appearance{selectedAppearances.length === 1 ? '' : 's'} at this occurrence</p>
      <div className="mt-4 grid gap-4">{selectedAppearances.map((appearance) => <AppearanceCard key={appearance.id} appearance={appearance} />)}</div>
    </section>}
    <section aria-labelledby="tour-appearances" className="mt-8"><h2 id="tour-appearances">Appearances</h2>
      <div className="mt-4 grid gap-4">{detail.appearances.map((appearance) => <AppearanceCard key={appearance.id} appearance={appearance} />)}</div>
    </section>
  </main>;
}
