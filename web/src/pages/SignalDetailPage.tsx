import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { useParams } from 'react-router-dom';
import { ConsentControl } from '../components/ConsentControl';
import { publicRequest } from '../lib/release-api';
import type { ConsentAction, ConsentScope, ConsentState } from '../types/audience';
import type { SignalDetailResponse } from '../types/signal';
import { PageMeta } from '../components/PageMeta';

const apiURL = import.meta.env.VITE_API_URL || '/api';
const consentFor = (detail: SignalDetailResponse) => detail.consent_scopes ?? detail.signal.consent_scopes ?? (detail.consent ?? detail.signal.consent ? [detail.consent ?? detail.signal.consent!] : []);

export function SignalDetailPage() {
  const { id = '' } = useParams();
  const query = useQuery({ queryKey: ['signal', id], queryFn: () => publicRequest<SignalDetailResponse>(`/signals/${id}`), enabled: Boolean(id) });
  const [localConsents, setLocalConsents] = useState<ConsentState[] | null>(null);
  const changeConsent = async (scope: ConsentScope, action: ConsentAction) => {
    const response = await fetch(`${apiURL}/audience/consent`, { method: 'POST', credentials: 'include', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ scope_id: scope.id, action }) });
    if (!response.ok) throw new Error('Consent update failed');
    const next = (localConsents ?? consentFor(query.data!)).map((item) => item.scope.id === scope.id ? { ...item, status: action === 'grant' ? 'granted' as const : 'revoked' as const } : item);
    setLocalConsents(next);
  };
  if (query.isPending) return <main className="content-wrap py-16" aria-busy="true"><p className="eyebrow">Loading signal</p></main>;
  if (query.isError || !query.data) return <main className="content-wrap py-16"><p className="eyebrow">Signal unavailable</p><h1 className="font-display mt-3 text-5xl uppercase">Transmission not found.</h1></main>;
  const { signal } = query.data;
  const consents = localConsents ?? consentFor(query.data);
  return <main className="content-wrap grid gap-8 py-12 lg:grid-cols-[1fr_360px]"><PageMeta title={signal.title}/>
    <article className="panel p-7 md:p-10"><div className="flex flex-wrap items-center gap-3"><p className="eyebrow">Signal // time-sensitive update</p><span className="status-chip">{signal.state}</span></div><h1 className="font-display mt-4 text-6xl uppercase">{signal.title}</h1><p className="mt-3 font-mono text-sm text-neon-cyan">From {signal.sender.name}</p>{signal.body && <p className="mt-8 whitespace-pre-wrap text-lg leading-8 text-foreground-secondary">{signal.body}</p>}{signal.target?.title && <div className="mt-8 border-l-2 border-neon-purple pl-4"><p className="eyebrow">Related to</p><p className="mt-1">{signal.target.title}</p></div>}</article>
    <aside className="panel h-fit p-6"><p className="eyebrow">Delivery controls</p><h2 className="font-display mt-2 text-3xl uppercase">Your permission, per channel</h2><p className="mt-3 text-sm leading-6 text-foreground-secondary">Viewing this Signal never opts you into messages. Every channel and purpose remains reversible.</p><div className="mt-5 grid gap-4">{consents.map((consent) => <ConsentControl key={consent.scope.id} consent={consent} onChange={changeConsent} />)}{consents.length === 0 && <p className="text-sm text-foreground-muted">No delivery request is attached.</p>}</div></aside>
  </main>;
}
