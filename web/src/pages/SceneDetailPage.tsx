import { useQuery } from '@tanstack/react-query';
import { Link, useParams } from 'react-router-dom';
import { getScene } from '../lib/release-api';
import { PageMeta } from '../components/PageMeta';

export function SceneDetailPage() {
  const { id = '' } = useParams();
  const query = useQuery({ queryKey: ['scene', id], queryFn: ({ signal }) => getScene(id, signal), enabled: Boolean(id) });
  if (query.isLoading) return <main className="content-wrap min-h-[70vh] py-16" aria-busy="true"><p className="eyebrow">Resolving scene…</p></main>;
  if (!query.data) return <main className="content-wrap min-h-[70vh] py-16"><p className="eyebrow">Scene unavailable</p><h1 className="font-display mt-3 text-6xl uppercase">Signal not found.</h1></main>;
  const scene = query.data;
  return <main><PageMeta title={scene.name}/>
    <header className="signal-grid border-b border-border py-16"><div className="content-wrap grid gap-10 lg:grid-cols-[1fr_320px]">
      <div><p className="eyebrow">Scene // {scene.visibility || 'public'}</p><h1 className="font-display mt-5 text-7xl font-bold uppercase leading-none">{scene.name}</h1><p className="mt-7 max-w-3xl text-xl leading-8 text-foreground-secondary">{scene.description || 'An independent local context for people, places, and sound.'}</p></div>
      <aside className="panel p-6"><span className="status-chip">Receiving</span><dl className="mt-6 grid gap-4 text-sm"><div><dt className="eyebrow">Location precision</dt><dd className="mt-1 text-foreground-secondary">{scene.allow_precise ? 'Public venue authority' : 'Coarse community area'}</dd></div><div><dt className="eyebrow">Tags</dt><dd className="mt-2 flex flex-wrap gap-2">{scene.tags?.map((tag) => <span className="status-chip" key={tag}>{tag}</span>) || 'Unclassified'}</dd></div></dl></aside>
    </div></header>
    <section className="content-wrap grid gap-8 py-12 lg:grid-cols-[1fr_320px]"><div><p className="eyebrow">Upcoming activity</p><h2 className="font-display mt-3 text-4xl uppercase">Dates connected to this scene</h2><div className="panel mt-6 p-8 text-foreground-secondary">No public dates are connected yet.</div></div><aside><Link to={`/search?q=${encodeURIComponent(scene.name)}&type=events`} className="button-secondary w-full">Search related dates</Link><p className="mt-4 text-sm leading-6 text-foreground-muted">Scene membership requests are not part of the durable public beta yet.</p></aside></section>
  </main>;
}
