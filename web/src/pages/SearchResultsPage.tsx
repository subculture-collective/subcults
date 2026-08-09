import { useEffect, useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Link, useSearchParams } from 'react-router-dom';
import { SearchBar } from '../components/SearchBar';
import { PageMeta } from '../components/PageMeta';
import { useSearch } from '../hooks/useSearch';
import { getAppearances } from '../lib/release-api';

type ResultType = 'all' | 'scenes' | 'events' | 'artists' | 'tours';
type Item = { type: Exclude<ResultType, 'all'>; id: string; title: string; description?: string; href: string };

export function SearchResultsPage() {
  const [params, setParams] = useSearchParams();
  const query = params.get('q')?.trim() ?? '';
  const requested = params.get('type');
  const type: ResultType = requested && ['scenes', 'events', 'artists', 'tours'].includes(requested) ? requested as ResultType : 'all';
  const searchState = useSearch({ limit: 30 });
  const appearances = useQuery({ queryKey: ['search-appearance-index'], queryFn: ({ signal }) => getAppearances({}, signal) });
  const { search, clear } = searchState;
  useEffect(() => { if (query) search(query); else clear(); }, [query, search, clear]);
  const items = useMemo<Item[]>(() => {
    const needle = query.toLocaleLowerCase();
    const matches = (value: string) => !needle || value.toLocaleLowerCase().includes(needle);
    const scenes: Item[] = searchState.results.scenes.map((scene) => ({ type: 'scenes', id: scene.id, title: scene.name, description: scene.description, href: `/scenes/${scene.id}` }));
    const events: Item[] = searchState.results.events.map((event) => ({ type: 'events', id: event.id, title: event.name, description: event.description, href: `/events/${event.id}` }));
    const artistMap = new Map<string, Item>(); const tourMap = new Map<string, Item>();
    for (const appearance of appearances.data ?? []) {
      if (matches(appearance.act.name)) artistMap.set(appearance.act.profile_id, { type: 'artists', id: appearance.act.profile_id, title: appearance.act.name, description: appearance.act.home_territory ? `Declared home territory: ${appearance.act.home_territory}` : 'Artist or creative project', href: `/profiles/${appearance.act.profile_id}` });
      if (appearance.tour && matches(appearance.tour.title)) tourMap.set(appearance.tour.id, { type: 'tours', id: appearance.tour.id, title: appearance.tour.title, description: `Tour featuring ${appearance.act.name}`, href: `/tours/${appearance.tour.id}` });
    }
    return [...scenes, ...events, ...artistMap.values(), ...tourMap.values()].filter((item) => type === 'all' || item.type === type);
  }, [appearances.data, query, searchState.results.events, searchState.results.scenes, type]);
  const chooseType = (next: ResultType) => { const copy = new URLSearchParams(params); if (next === 'all') copy.delete('type'); else copy.set('type', next); setParams(copy, { replace: true }); };
  const loading = searchState.loading || appearances.isLoading;
  const error = searchState.error || appearances.isError;
  return <main><PageMeta title="Search artists, tours, events, and scenes"/><header className="border-b border-border bg-surface/60"><div className="content-wrap py-12"><p className="eyebrow">Public discovery</p><h1 className="font-display mt-2 text-6xl uppercase">Search Subcult</h1><p className="mt-4 max-w-2xl text-foreground-secondary">Find artists, tours, shows, festivals, venues, and scenes. Protected and unlisted activity stays out of public search.</p><div className="mt-6 max-w-2xl"><SearchBar /></div></div></header><div className="content-wrap grid gap-8 py-10 lg:grid-cols-[220px_1fr]"><aside aria-label="Search filters"><p className="eyebrow">Show me</p><div className="panel mt-4 grid p-2">{(['all', 'artists', 'tours', 'events', 'scenes'] as ResultType[]).map((option) => <button key={option} aria-pressed={type === option} className={`border-l-2 px-4 py-3 text-left font-mono text-xs uppercase tracking-wider ${type === option ? 'border-neon-green bg-background-hover text-neon-green' : 'border-transparent text-foreground-muted hover:text-foreground'}`} onClick={() => chooseType(option)}>{option}</button>)}</div></aside><section aria-live="polite"><div className="flex items-end justify-between"><div><p className="eyebrow">Results</p><h2 className="font-display mt-2 text-4xl uppercase">{query ? `“${query}”` : type === 'all' ? 'Browse the public index' : `Browse ${type}`}</h2></div>{!loading && <span className="status-chip">{items.length} found</span>}</div>{loading && <div className="panel mt-6 p-8" role="status" aria-busy="true">Searching public records…</div>}{error && <div className="panel mt-6 border-danger p-8 text-danger">Search is temporarily unavailable.</div>}<div className="mt-6 grid gap-3">{items.map((item) => <Link key={`${item.type}-${item.id}`} to={item.href} className="panel group flex items-center justify-between gap-4 p-5 hover:border-neon-purple"><div><p className="eyebrow">{item.type.slice(0, -1)}</p><h3 className="font-display mt-1 text-3xl uppercase group-hover:text-neon-cyan">{item.title}</h3>{item.description && <p className="mt-2 line-clamp-2 text-sm text-foreground-secondary">{item.description}</p>}</div><span className="font-mono text-neon-purple" aria-hidden="true">↗</span></Link>)}{!loading && items.length === 0 && <div className="panel p-8 text-foreground-secondary">No public records match this search. Try another name, city, or filter.</div>}</div></section></div></main>;
}
