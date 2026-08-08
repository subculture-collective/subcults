import { useEffect } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { SearchBar } from '../components/SearchBar';
import { useSearch } from '../hooks/useSearch';
import type { SearchResultItem } from '../types/search';

type ResultType = 'all' | 'scenes' | 'events' | 'posts';

function resultPath(item: SearchResultItem) {
  if (item.type === 'scene') return `/scenes/${item.data.id}`;
  if (item.type === 'event') return `/events/${item.data.id}`;
  return `/posts/${item.data.id}`;
}

function resultTitle(item: SearchResultItem) {
  if (item.type === 'scene') return item.data.name;
  if (item.type === 'event') return item.data.name;
  return item.data.title || item.data.content?.slice(0, 80) || 'Untitled post';
}

export function SearchResultsPage() {
  const [params, setParams] = useSearchParams();
  const query = params.get('q') ?? '';
  const type = (params.get('type') as ResultType) || 'all';
  const searchState = useSearch({ limit: 30 });
  const { search, clear } = searchState;
  useEffect(() => { if (query.trim()) search(query); else clear(); }, [query, search, clear]);
  const items: SearchResultItem[] = [
    ...searchState.results.scenes.map((data) => ({ type: 'scene' as const, data })),
    ...searchState.results.events.map((data) => ({ type: 'event' as const, data })),
    ...searchState.results.posts.map((data) => ({ type: 'post' as const, data })),
  ].filter((item) => type === 'all' || `${item.type}s` === type);
  const chooseType = (next: ResultType) => { const copy = new URLSearchParams(params); if (next === 'all') copy.delete('type'); else copy.set('type', next); setParams(copy, { replace: true }); };
  return <main>
    <header className="border-b border-border bg-surface/60"><div className="content-wrap py-12"><p className="eyebrow">Cross-scene index</p><h1 className="font-display mt-2 text-6xl uppercase">Search</h1><div className="mt-6 max-w-2xl"><SearchBar /></div></div></header>
    <div className="content-wrap grid gap-8 py-10 lg:grid-cols-[220px_1fr]">
      <aside><p className="eyebrow">Filter index</p><div className="panel mt-4 grid p-2">{(['all', 'scenes', 'events', 'posts'] as ResultType[]).map((option) => <button key={option} className={`border-l-2 px-4 py-3 text-left font-mono text-xs uppercase tracking-wider ${type === option ? 'border-neon-green bg-background-hover text-neon-green' : 'border-transparent text-foreground-muted hover:text-foreground'}`} onClick={() => chooseType(option)}>{option}</button>)}</div></aside>
      <section aria-live="polite"><div className="flex items-end justify-between"><div><p className="eyebrow">Query</p><h2 className="font-display mt-2 text-4xl uppercase">{query ? `“${query}”` : 'Enter a search'}</h2></div>{query && <span className="status-chip">{items.length} found</span>}</div>
        {searchState.loading && <div className="panel mt-6 p-8" aria-busy="true">Resolving the index…</div>}
        {searchState.error && <div className="panel mt-6 border-danger p-8 text-danger">Search is temporarily unavailable.</div>}
        <div className="mt-6 grid gap-3">{items.map((item) => <Link key={`${item.type}-${item.data.id}`} to={resultPath(item)} className="panel group flex items-center justify-between gap-4 p-5 hover:border-neon-purple"><div><p className="eyebrow">{item.type}</p><h3 className="font-display mt-1 text-3xl uppercase group-hover:text-neon-cyan">{resultTitle(item)}</h3></div><span className="font-mono text-neon-purple" aria-hidden="true">↗</span></Link>)}{query && !searchState.loading && items.length === 0 && <div className="panel p-8 text-foreground-secondary">No matching public records. Protected and unlisted activity stays out of the index.</div>}</div>
      </section>
    </div>
  </main>;
}
