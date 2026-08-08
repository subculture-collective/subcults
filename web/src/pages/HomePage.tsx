import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { EntityCard } from '../components/release/EntityCard';
import { getAppearances } from '../lib/release-api';

const markets = ['Chicago', 'Detroit', 'New York', 'Los Angeles', 'London'];

export function HomePage() {
  const [market, setMarket] = useState(markets[0]);
  const [mode, setMode] = useState<'list' | 'map'>('list');
  const appearances = useQuery({ queryKey: ['appearances', market], queryFn: ({ signal }) => getAppearances(signal) });
  return <>
    <section className="signal-grid border-b border-border">
      <div className="content-wrap grid min-h-[430px] items-end gap-10 py-14 lg:grid-cols-[1.2fr_.8fr] lg:py-20">
        <div>
          <p className="eyebrow">Public frequency // Beta transmission</p>
          <h1 className="font-display mt-5 max-w-4xl text-6xl font-bold uppercase leading-[.84] tracking-tight text-foreground sm:text-7xl lg:text-[7rem]">
            Find the scene<br/><span className="text-neon-purple">before the feed does.</span>
          </h1>
          <p className="mt-8 max-w-2xl text-lg leading-8 text-foreground-secondary">Local rooms, visiting artists, festival appearances, one-off transmissions, and the communities keeping them alive.</p>
        </div>
        <aside className="panel panel-cut p-6 lg:mb-2">
          <p className="eyebrow">Current receiving area</p>
          <label className="mt-5 block text-sm text-foreground-secondary" htmlFor="market">Choose a launch market</label>
          <select id="market" value={market} onChange={(event) => setMarket(event.target.value)} className="field mt-2">
            {markets.map((item) => <option key={item}>{item}</option>)}
          </select>
          <p className="font-mono mt-5 text-xs leading-6 text-foreground-muted">Location is never requested automatically. Browse a market or opt into approximate device location later.</p>
        </aside>
      </div>
    </section>

    <section className="content-wrap py-10 lg:py-14">
      <div className="flex flex-col justify-between gap-5 border-b border-border pb-6 md:flex-row md:items-end">
        <div><p className="eyebrow">Incoming dates</p><h2 className="font-display mt-2 text-4xl font-semibold uppercase tracking-wide">Around {market}</h2></div>
        <div className="flex gap-2" role="group" aria-label="Discovery view">
          <button className={mode === 'list' ? 'button-primary' : 'button-secondary'} onClick={() => setMode('list')}>List</button>
          <button className={mode === 'map' ? 'button-primary' : 'button-secondary'} onClick={() => setMode('map')}>Map</button>
        </div>
      </div>
      <div className="mt-5 flex flex-wrap gap-2" aria-label="Quick filters">
        {['Tonight', 'This week', 'Tour stops', 'Festivals', 'Visiting', 'Accessible'].map((filter) => <button key={filter} className="button-quiet border border-border">{filter}</button>)}
      </div>

      {mode === 'map' ? <div className="signal-grid panel mt-7 grid min-h-[520px] place-items-center overflow-hidden">
        <div className="max-w-md p-8 text-center"><p className="eyebrow">Map receiver</p><h3 className="font-display mt-3 text-4xl uppercase">Occurrence layer ready</h3><p className="mt-4 text-foreground-secondary">The live map renders server-approved public points only. Configure the MapTiler key to activate tiles.</p></div>
      </div> : <div className="mt-7 grid gap-4 md:grid-cols-2 xl:grid-cols-3">
        {appearances.isLoading && Array.from({ length: 6 }, (_, index) => <div key={index} className="panel h-64 animate-pulse bg-surface" />)}
        {appearances.isError && <div className="panel col-span-full p-8"><p className="eyebrow">Receiver offline</p><p className="mt-3 text-foreground-secondary">Discovery data could not be reached. The public shell remains available while the signal recovers.</p></div>}
        {appearances.data?.map((appearance) => <EntityCard
          key={appearance.id}
          href={`/events/${appearance.event.id}`}
          eyebrow={appearance.context.replaceAll('_', ' ')}
          title={appearance.event.title}
          description={`${appearance.act.name}${appearance.act.home_territory ? ` from ${appearance.act.home_territory}` : ''}`}
          meta={[new Date(appearance.event.starts_at).toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' }), appearance.locality, ...appearance.host_names.slice(0, 1)]}
          status={appearance.status}
          accent={appearance.event.kind === 'festival' ? 'orange' : appearance.locality === 'visiting' ? 'cyan' : 'purple'}
        />)}
        {appearances.isSuccess && appearances.data.length === 0 && <div className="panel col-span-full grid min-h-64 place-items-center p-8 text-center"><div><p className="eyebrow">No verified dates yet</p><h3 className="font-display mt-3 text-4xl uppercase">Help tune this market.</h3><p className="mt-3 text-foreground-secondary">Approved launch partners can publish or import the first dates.</p><Link className="button-primary mt-6" to="/creator-access">Request Studio access</Link></div></div>}
      </div>}
    </section>

    <section className="border-y border-border bg-background-secondary/60 py-14">
      <div className="content-wrap grid gap-8 lg:grid-cols-3">
        {[['01', 'Discovery without surveillance', 'Choose a place. Device location remains optional and approximate.'], ['02', 'Context over follower count', 'Scenes, hosts, appearances, and provenance explain why a date matters.'], ['03', 'Consent is a record', 'RSVP and membership never silently become marketing permission.']].map(([number, title, copy]) => <article key={number} className="dossier-rule pt-6"><span className="font-mono text-xs text-neon-purple">[{number}]</span><h3 className="font-display mt-3 text-3xl uppercase">{title}</h3><p className="mt-3 leading-7 text-foreground-secondary">{copy}</p></article>)}
      </div>
    </section>
  </>;
}
