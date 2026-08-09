import { useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { EntityCard } from '../components/release/EntityCard';
import { PageMeta } from '../components/PageMeta';
import { getAppearances, type AppearanceFilters } from '../lib/release-api';

type Filter = 'Today' | '7 days' | '30 days' | 'Shows' | 'Festivals' | 'One-offs' | 'Tour stops';
export function EventsPage() {
  const [active, setActive] = useState<Filter | null>(null);
  const filters = useMemo<AppearanceFilters>(() => {
    const from = new Date(); const next: AppearanceFilters = { from: from.toISOString() };
    const days = active === 'Today' ? 1 : active === '7 days' ? 7 : active === '30 days' ? 30 : 0;
    if (days) next.to = new Date(from.getTime() + days * 864e5).toISOString();
    if (active === 'Festivals') next.festival = true;
    if (active === 'Shows') next.kind = 'show';
    return next;
  }, [active]);
  const query = useQuery({ queryKey: ['all-appearances', filters], queryFn: ({ signal }) => getAppearances(filters, signal) });
  const dates = query.data?.filter((item) => active !== 'One-offs' || item.context === 'one_off').filter((item) => active !== 'Tour stops' || item.context === 'tour_stop') ?? [];
  return <main className="content-wrap py-12 lg:py-16"><PageMeta title="Shows, festivals, and tour dates"/><p className="eyebrow">Public dates</p><h1 className="font-display mt-4 text-6xl font-bold uppercase sm:text-8xl">Shows, festivals,<br/><span className="text-neon-purple">and tour dates.</span></h1><p className="mt-6 max-w-2xl text-lg leading-8 text-foreground-secondary">Browse verified public appearances without exposing protected venue details.</p><div className="mt-8 flex flex-wrap gap-2">{(['Today', '7 days', '30 days', 'Shows', 'Festivals', 'One-offs', 'Tour stops'] as Filter[]).map((item) => <button aria-pressed={active === item} className={active === item ? 'button-primary' : 'button-quiet border border-border'} onClick={() => setActive(active === item ? null : item)} key={item}>{item}</button>)}</div><section className="mt-9 grid gap-4 md:grid-cols-2 xl:grid-cols-3" aria-label="Public events">{query.isLoading && Array.from({ length: 6 }, (_, index) => <div className="panel h-60 animate-pulse" key={index} />)}{query.isError && <div className="panel col-span-full p-8 text-foreground-secondary">We couldn’t load public dates. Try again in a moment.</div>}{dates.map((appearance) => <EntityCard key={appearance.id} href={`/events/${appearance.event.id}`} eyebrow={appearance.context.replaceAll('_', ' ')} title={appearance.event.title} description={appearance.act.name} status={appearance.status} meta={[new Date(appearance.event.starts_at).toLocaleDateString(), appearance.locality]} accent={appearance.event.kind === 'festival' ? 'orange' : 'purple'} />)}{query.isSuccess && dates.length === 0 && <div className="panel col-span-full p-10 text-foreground-secondary">No public dates match this filter yet.</div>}</section></main>;
}
