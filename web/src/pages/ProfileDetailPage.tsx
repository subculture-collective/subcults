import { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { AppearanceCard } from '../components/discovery/AppearanceCard';
import type { TouringDetailResponse } from '../types/touring';

function readDetail(response: unknown): TouringDetailResponse {
  const value = response as TouringDetailResponse & { data?: TouringDetailResponse };
  return value.data ?? value;
}

export function ProfileDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [detail, setDetail] = useState<TouringDetailResponse | null>(null);
  const [error, setError] = useState(false);

  useEffect(() => {
    if (!id) return;
    let active = true;
    fetch(`${import.meta.env.VITE_API_URL || '/api'}/profiles/${id}`)
      .then(async (response) => response.ok ? readDetail(await response.json()) : Promise.reject(response))
      .then((result) => { if (active) setDetail(result); })
      .catch(() => { if (active) setError(true); });
    return () => { active = false; };
  }, [id]);

  if (error) return <main className="p-8"><h1>Profile unavailable</h1><p>Touring details could not be loaded.</p></main>;
  if (!detail) return <main className="p-8" aria-busy="true"><h1>Loading profile…</h1></main>;
  const profile = detail.profile;
  return <main className="mx-auto max-w-4xl p-6 md:p-8">
    <p className="font-mono text-xs uppercase tracking-wide text-neon-cyan">Artist profile</p>
    <h1 className="mt-1">{profile?.name ?? 'Profile'}</h1>
    {profile?.home_territory && <p className="text-foreground-secondary">Home territory: {profile.home_territory}</p>}
    <section aria-labelledby="profile-appearances" className="mt-8">
      <h2 id="profile-appearances">Upcoming appearances</h2>
      <div className="mt-4 grid gap-4">{detail.appearances.map((appearance) => <AppearanceCard key={appearance.id} appearance={appearance} />)}</div>
    </section>
  </main>;
}
