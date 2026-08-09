import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import type { Map as MapLibreMap } from 'maplibre-gl';
import { useParams } from 'react-router-dom';
import { AppearanceCard } from '../components/discovery/AppearanceCard';
import { TourMapLayer } from '../components/discovery/TourMapLayer';
import { MapView } from '../components/MapView';
import { publicRequest } from '../lib/release-api';
import type { TouringDetailResponse } from '../types/touring';
import { PortableRecordBadge } from '../components/discovery/PortableRecordBadge';

export function TourDetailPage() {
  const { id = '' } = useParams();
  const [map, setMap] = useState<MapLibreMap | null>(null);
  const [selectedEventID, setSelectedEventID] = useState<string | null>(null);
  const { data, isPending, isError } = useQuery({ queryKey: ['tour', id], queryFn: () => publicRequest<TouringDetailResponse>(`/tours/${id}`), enabled: Boolean(id) });
  if (isPending) return <main className="content-wrap py-16" aria-busy="true"><p className="eyebrow">Loading itinerary</p></main>;
  if (isError || !data) return <main className="content-wrap py-16"><p className="eyebrow">Tour unavailable</p><h1 className="font-display mt-3 text-5xl uppercase">This itinerary is off-air.</h1></main>;
  const selected = selectedEventID ? data.appearances.filter((appearance) => appearance.event.id === selectedEventID) : [];
  return <main>
    <header className="border-b border-border bg-surface/60"><div className="content-wrap py-12"><div className="flex flex-wrap items-center gap-3"><p className="eyebrow">Tour itinerary</p><span className="status-chip">{data.appearances.length} appearances</span></div><h1 className="font-display mt-3 text-6xl uppercase md:text-8xl">{data.tour?.title ?? 'Untitled tour'}</h1><PortableRecordBadge record={data.tour}/></div></header>
    <div className="content-wrap grid gap-8 py-10 lg:grid-cols-[1.15fr_.85fr]">
      <section className="panel relative min-h-[420px] overflow-hidden" aria-label="Tour occurrence map"><MapView className="absolute inset-0 h-full" onLoad={setMap} /><TourMapLayer map={map} appearances={data.appearances} onSelectEvent={setSelectedEventID} /></section>
      <section aria-labelledby="tour-dates"><p className="eyebrow">Chronology</p><h2 id="tour-dates" className="font-display mt-2 text-4xl uppercase">Dates</h2><div className="mt-5 grid max-h-[620px] gap-4 overflow-y-auto pr-1">{data.appearances.map((appearance) => <AppearanceCard key={appearance.id} appearance={appearance} />)}</div></section>
    </div>
    {selected.length > 0 && <aside className="fixed inset-x-4 bottom-4 z-30 panel mx-auto max-w-2xl p-5 shadow-2xl" aria-live="polite"><div className="flex items-start justify-between gap-4"><div><p className="eyebrow">Selected occurrence</p><h2 className="font-display mt-1 text-3xl uppercase">{selected[0].event.title}</h2></div><button className="button-secondary" onClick={() => setSelectedEventID(null)}>Close</button></div><p className="mt-3 text-sm text-foreground-secondary">{selected.length} billed appearance{selected.length === 1 ? '' : 's'} at this event.</p></aside>}
  </main>;
}
