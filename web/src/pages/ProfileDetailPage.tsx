import { useQuery } from '@tanstack/react-query';
import { useParams } from 'react-router-dom';
import { AppearanceCard } from '../components/discovery/AppearanceCard';
import { publicRequest } from '../lib/release-api';
import type { TouringDetailResponse } from '../types/touring';
import { PortableRecordBadge } from '../components/discovery/PortableRecordBadge';

export function ProfileDetailPage() {
  const { id = '' } = useParams();
  const { data, isPending, isError } = useQuery({ queryKey: ['profile', id], queryFn: () => publicRequest<TouringDetailResponse>(`/profiles/${id}`), enabled: Boolean(id) });
  if (isPending) return <main className="content-wrap py-16" aria-busy="true"><p className="eyebrow">Loading profile</p></main>;
  if (isError || !data) return <main className="content-wrap py-16"><p className="eyebrow">Profile unavailable</p><h1 className="font-display mt-3 text-5xl uppercase">This profile is off-air.</h1></main>;
  return <main>
    <header className="border-b border-border bg-surface/60"><div className="content-wrap py-14"><p className="eyebrow">Artist / project profile</p><h1 className="font-display mt-3 text-6xl uppercase md:text-8xl">{data.profile?.name ?? 'Untitled profile'}</h1>{data.profile?.home_territory && <p className="mt-4 font-mono text-sm text-foreground-secondary">Declared home territory · {data.profile.home_territory}</p>}<PortableRecordBadge record={data.profile}/></div></header>
    <section className="content-wrap py-10" aria-labelledby="appearances"><div className="mb-6 flex items-end justify-between"><div><p className="eyebrow">Movement</p><h2 id="appearances" className="font-display mt-2 text-4xl uppercase">Upcoming appearances</h2></div><span className="status-chip">{data.appearances.length} listed</span></div><div className="grid gap-4">{data.appearances.map((appearance) => <AppearanceCard key={appearance.id} appearance={appearance} />)}{data.appearances.length === 0 && <div className="panel p-8 text-foreground-secondary">No announced dates. Nothing here implies the artist is inactive or local.</div>}</div></section>
  </main>;
}
