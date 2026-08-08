import { useQuery } from '@tanstack/react-query';
import { useParams } from 'react-router-dom';
import { getEvent } from '../lib/release-api';

export function EventDetailPage() {
  const { id = '' } = useParams();
  const query = useQuery({ queryKey: ['event', id], queryFn: ({ signal }) => getEvent(id, signal), enabled: Boolean(id) });
  if (query.isLoading) return <main className="content-wrap min-h-[70vh] py-16" aria-busy="true"><p className="eyebrow">Resolving occurrence…</p></main>;
  if (!query.data) return <main className="content-wrap min-h-[70vh] py-16"><p className="eyebrow">Occurrence unavailable</p><h1 className="font-display mt-3 text-6xl uppercase">Date not found.</h1></main>;
  const event = query.data;
  const title = event.title || event.name;
  return <main>
    <header className="signal-grid border-b border-border py-16"><div className="content-wrap">
      <div className="flex flex-wrap items-center gap-3"><span className="status-chip">{event.status || 'announced'}</span><span className="eyebrow">{event.kind || 'show'}</span></div>
      <h1 className="font-display mt-6 max-w-5xl text-6xl font-bold uppercase leading-[.9] sm:text-8xl">{title}</h1>
      {event.starts_at && <p className="font-mono mt-7 text-sm uppercase tracking-wider text-neon-cyan">{new Date(event.starts_at).toLocaleString(undefined, { dateStyle: 'full', timeStyle: 'short' })}</p>}
    </div></header>
    <section className="content-wrap grid gap-8 py-12 lg:grid-cols-[1fr_360px]"><div><p className="eyebrow">Occurrence dossier</p><p className="mt-5 max-w-3xl text-lg leading-8 text-foreground-secondary">{event.description || 'Details are maintained by the event hosts and preserved with source provenance.'}</p><div className="dossier-rule mt-10 pt-6"><p className="eyebrow">Location disclosure</p><p className="mt-3 text-foreground-secondary">{event.occurrence?.precision === 'precise' ? 'Exact public venue' : 'Approximate area. Protected details require authorization.'}</p></div></div><aside className="panel p-6"><button className="button-primary w-full">RSVP // Going</button><button className="button-secondary mt-3 w-full">Maybe</button><p className="font-mono mt-5 text-[.62rem] leading-5 text-foreground-muted">RSVP records participation only. It does not grant email or push consent.</p></aside></section>
  </main>;
}
