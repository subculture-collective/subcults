import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { EntityCard } from '../components/release/EntityCard';
import { publicRequest } from '../lib/release-api';
import type { Scene } from '../types/scene';

export function ScenesPage() {
  const query = useQuery({ queryKey: ['public-scenes'], queryFn: ({ signal }) => publicRequest<{ results: Scene[] }>('/search/scenes?q=&limit=24', signal) });
  const scenes = query.data?.results || [];
  return <main>
    <header className="signal-grid border-b border-border py-16"><div className="content-wrap"><p className="eyebrow">Community contexts</p><h1 className="font-display mt-4 max-w-5xl text-6xl font-bold uppercase leading-[.9] sm:text-8xl">Not genres.<br/><span className="text-neon-purple">Living systems.</span></h1><p className="mt-7 max-w-2xl text-lg leading-8 text-foreground-secondary">Scenes connect people, rooms, artists, and local memory without pretending they belong to an algorithm.</p></div></header>
    <section className="content-wrap py-12"><div className="flex items-end justify-between gap-5"><div><p className="eyebrow">Public registry</p><h2 className="font-display mt-2 text-4xl uppercase">Receiving scenes</h2></div><Link className="button-secondary" to="/creator-access">Start a scene</Link></div>
      <div className="mt-8 grid gap-4 md:grid-cols-2 xl:grid-cols-3">{query.isLoading && Array.from({ length: 6 }, (_, index) => <div className="panel h-56 animate-pulse" key={index} />)}{scenes.map((scene) => <EntityCard key={scene.id} href={`/scenes/${scene.id}`} eyebrow={scene.visibility || 'public'} title={scene.name} description={scene.description} meta={scene.tags?.slice(0, 3)} accent="cyan" />)}{query.isSuccess && scenes.length === 0 && <div className="panel col-span-full p-10 text-foreground-secondary">The registry is awaiting its first approved partner scenes.</div>}</div>
    </section>
  </main>;
}
