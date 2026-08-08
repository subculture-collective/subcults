import { useQuery } from '@tanstack/react-query';
import { EntityCard } from '../components/release/EntityCard';
import { getAppearances } from '../lib/release-api';

export function EventsPage() {
  const query = useQuery({ queryKey: ['all-appearances'], queryFn: ({ signal }) => getAppearances(signal) });
  return <main className="content-wrap py-12 lg:py-16">
    <p className="eyebrow">Occurrence index</p><h1 className="font-display mt-4 text-6xl font-bold uppercase sm:text-8xl">Dates in motion.</h1>
    <div className="mt-8 flex flex-wrap gap-2">{['Today', '7 days', '30 days', 'Shows', 'Festivals', 'One-offs', 'Tour stops'].map((item) => <button className="button-quiet border border-border" key={item}>{item}</button>)}</div>
    <section className="mt-9 grid gap-4 md:grid-cols-2 xl:grid-cols-3" aria-label="Public events">
      {query.isLoading && Array.from({ length: 6 }, (_, index) => <div className="panel h-60 animate-pulse" key={index} />)}
      {query.data?.map((appearance) => <EntityCard key={appearance.id} href={`/events/${appearance.event.id}`} eyebrow={appearance.context.replaceAll('_', ' ')} title={appearance.event.title} description={appearance.act.name} status={appearance.status} meta={[new Date(appearance.event.starts_at).toLocaleDateString(), appearance.locality]} accent={appearance.event.kind === 'festival' ? 'orange' : 'purple'} />)}
      {query.isSuccess && query.data.length === 0 && <div className="panel col-span-full p-10 text-foreground-secondary">No verified public dates have entered the index.</div>}
    </section>
  </main>;
}
