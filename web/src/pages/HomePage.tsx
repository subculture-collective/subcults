import { useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import type { Map as MapLibreMap } from 'maplibre-gl';
import { EntityCard } from '../components/release/EntityCard';
import { MapView } from '../components/MapView';
import { TourMapLayer } from '../components/discovery/TourMapLayer';
import { PageMeta } from '../components/PageMeta';
import { getAppearances, type AppearanceFilters } from '../lib/release-api';

const markets = {
  Chicago: '-88.10,41.60,-87.45,42.10', Detroit: '-83.35,42.15,-82.85,42.55',
  'New York': '-74.30,40.45,-73.65,40.95', 'Los Angeles': '-118.70,33.65,-117.80,34.35', London: '-0.55,51.25,0.35,51.75',
};
type Filter = 'Tonight' | 'This week' | 'Tour stops' | 'Festivals' | 'Visiting';

function discoveryFilters(market: keyof typeof markets, active: Filter | null): AppearanceFilters {
  const now = new Date();
  const filters: AppearanceFilters = { bbox: markets[market], from: now.toISOString() };
  if (active === 'Tonight') { const end = new Date(now); end.setHours(23, 59, 59, 999); filters.to = end.toISOString(); }
  if (active === 'This week') filters.to = new Date(now.getTime() + 7 * 864e5).toISOString();
  if (active === 'Festivals') filters.festival = true;
  if (active === 'Visiting') filters.locality = 'visiting';
  return filters;
}

export function HomePage() {
  const [market, setMarket] = useState<keyof typeof markets>('Chicago');
  const [mode, setMode] = useState<'list' | 'map'>('list');
  const [active, setActive] = useState<Filter | null>(null);
  const [map, setMap] = useState<MapLibreMap | null>(null);
  const filters = useMemo(() => discoveryFilters(market, active), [market, active]);
  const appearances = useQuery({ queryKey: ['appearances', filters], queryFn: ({ signal }) => getAppearances(filters, signal) });
  const visible = appearances.data?.filter((item) => active !== 'Tour stops' || item.context === 'tour_stop') ?? [];
  return <>
    <PageMeta title="Underground shows near you" />
    <section className="signal-grid border-b border-border">
      <div className="content-wrap grid min-h-[480px] items-end gap-10 py-14 lg:grid-cols-[1.2fr_.8fr] lg:py-20">
        <div><p className="eyebrow">Underground shows, by city</p><h1 className="font-display mt-5 max-w-4xl text-6xl font-bold uppercase leading-[.84] tracking-tight text-foreground sm:text-7xl lg:text-[7rem]">Find the shows<br/><span className="text-neon-purple">your feed misses.</span></h1><p className="mt-8 max-w-2xl text-lg leading-8 text-foreground-secondary">Discover local scenes, tour stops, festivals, and one-off shows near you—without sharing your exact location.</p><div className="mt-8 flex flex-wrap gap-3"><a className="button-primary" href="#discover">Explore shows in {market}</a><Link className="button-secondary" to="/creator-access">Publish a show</Link></div></div>
        <aside className="panel panel-cut p-6 lg:mb-2"><p className="eyebrow">Browse a city</p><label className="mt-5 block text-sm text-foreground-secondary" htmlFor="market">Discovery area</label><select id="market" value={market} onChange={(event) => setMarket(event.target.value as keyof typeof markets)} className="field mt-2">{Object.keys(markets).map((item) => <option key={item}>{item}</option>)}</select><p className="font-mono mt-5 text-xs leading-6 text-foreground-muted">Subcults never requests your location automatically. Choose a city now; approximate device location remains optional.</p></aside>
      </div>
    </section>
    <section id="discover" className="content-wrap scroll-mt-24 py-10 lg:py-14">
      <div className="flex flex-col justify-between gap-5 border-b border-border pb-6 md:flex-row md:items-end"><div><p className="eyebrow">Upcoming</p><h2 className="font-display mt-2 text-4xl font-semibold uppercase tracking-wide">Shows around {market}</h2></div><div className="flex gap-2" role="group" aria-label="Discovery view"><button className={mode === 'list' ? 'button-primary' : 'button-secondary'} onClick={() => setMode('list')}>List</button><button className={mode === 'map' ? 'button-primary' : 'button-secondary'} onClick={() => setMode('map')}>Map</button></div></div>
      <div className="mt-5 flex flex-wrap gap-2" aria-label="Quick filters">{(['Tonight', 'This week', 'Tour stops', 'Festivals', 'Visiting'] as Filter[]).map((filter) => <button aria-pressed={active === filter} key={filter} className={active === filter ? 'button-primary' : 'button-quiet border border-border'} onClick={() => setActive(active === filter ? null : filter)}>{filter}</button>)}</div>
      {mode === 'map' ? <div className="panel relative mt-7 min-h-[520px] overflow-hidden"><MapView className="absolute inset-0 h-full" initialPosition={{ bounds: markets[market].split(',').map(Number) as [number, number, number, number] }} onLoad={setMap}/><TourMapLayer map={map} appearances={visible}/></div> : <div className="mt-7 grid gap-4 md:grid-cols-2 xl:grid-cols-3">
        {appearances.isLoading && Array.from({ length: 6 }, (_, index) => <div key={index} className="panel h-64 animate-pulse bg-surface" />)}
        {appearances.isError && <div className="panel col-span-full p-8"><p className="eyebrow">Discovery unavailable</p><p className="mt-3 text-foreground-secondary">We couldn’t load public dates. Try again in a moment.</p></div>}
        {visible.map((appearance) => <EntityCard key={appearance.id} href={`/events/${appearance.event.id}`} eyebrow={appearance.context.replaceAll('_', ' ')} title={appearance.event.title} description={`${appearance.act.name}${appearance.act.home_territory ? ` from ${appearance.act.home_territory}` : ''}`} meta={[new Date(appearance.event.starts_at).toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' }), appearance.locality, ...appearance.host_names.slice(0, 1)]} status={appearance.status} accent={appearance.event.kind === 'festival' ? 'orange' : appearance.locality === 'visiting' ? 'cyan' : 'purple'} />)}
        {appearances.isSuccess && visible.length === 0 && <div className="panel col-span-full grid min-h-64 place-items-center p-8 text-center"><div><p className="eyebrow">No dates listed yet</p><h3 className="font-display mt-3 text-4xl uppercase">Know what’s happening?</h3><p className="mt-3 text-foreground-secondary">Artists, venues, promoters, and scene organizers can request beta publishing access.</p><Link className="button-primary mt-6" to="/creator-access">Publish a date</Link></div></div>}
      </div>}
    </section>
    <section className="border-y border-border bg-background-secondary/60 py-14"><div className="content-wrap"><p className="eyebrow">How discovery works</p><div className="mt-7 grid gap-8 lg:grid-cols-3">{[['01', 'Choose a place', 'Browse a city without giving Subcults your device location.'], ['02', 'See the context', 'Hosts, artists, tour relationships, and source history explain why a date matters.'], ['03', 'Control every message', 'Saving a date never becomes email, push, or marketing permission.']].map(([number, title, copy]) => <article key={number} className="dossier-rule pt-6"><span className="font-mono text-xs text-neon-purple">[{number}]</span><h3 className="font-display mt-3 text-3xl uppercase">{title}</h3><p className="mt-3 leading-7 text-foreground-secondary">{copy}</p></article>)}</div></div></section>
    <section className="content-wrap grid gap-5 py-14 md:grid-cols-2"><article className="panel p-7"><p className="eyebrow">For fans</p><h2 className="font-display mt-3 text-4xl uppercase">Follow the work, not the algorithm.</h2><p className="mt-4 text-foreground-secondary">Find artists across cities, save dates, and choose exactly which updates reach you.</p><Link className="button-secondary mt-6" to="/events">Browse all dates</Link></article><article className="panel p-7"><p className="eyebrow">For creators and organizers</p><h2 className="font-display mt-3 text-4xl uppercase">Publish once. Keep the context.</h2><p className="mt-4 text-foreground-secondary">Connect artists, scenes, venues, festivals, and tours without flattening them into a campaign list.</p><Link className="button-primary mt-6" to="/creator-access">Request Studio access</Link></article></section>
  </>;
}
