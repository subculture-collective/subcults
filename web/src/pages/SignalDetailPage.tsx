import { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { ConsentControl } from '../components/ConsentControl';
import type { ConsentAction, ConsentScope, ConsentState } from '../types/audience';
import type { Signal, SignalDetailResponse } from '../types/signal';

const apiURL = import.meta.env.VITE_API_URL || '/api';

function readDetail(response: unknown): SignalDetailResponse {
  const value = response as SignalDetailResponse & { data?: SignalDetailResponse };
  const detail = value.data ?? value;
  return 'signal' in detail ? detail : { signal: detail as unknown as Signal };
}

function consentFor(detail: SignalDetailResponse): ConsentState[] {
  return detail.consent_scopes ?? detail.signal.consent_scopes ?? (detail.consent ?? detail.signal.consent ? [detail.consent ?? detail.signal.consent!] : []);
}

export function SignalDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [detail, setDetail] = useState<SignalDetailResponse | null>(null);
  const [error, setError] = useState(false);

  useEffect(() => {
    if (!id) return;
    let active = true;
    fetch(`${apiURL}/signals/${id}`)
      .then(async (response) => response.ok ? readDetail(await response.json()) : Promise.reject(response))
      .then((result) => { if (active) setDetail(result); })
      .catch(() => { if (active) setError(true); });
    return () => { active = false; };
  }, [id]);

  const changeConsent = async (scope: ConsentScope, action: ConsentAction) => {
    const response = await fetch(`${apiURL}/audience/consent`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ scope_id: scope.id, action }),
    });
    if (!response.ok) throw new Error('Consent update failed');
    const updated = await response.json().catch(() => null) as { consent?: ConsentState } | null;
    setDetail((current) => {
      if (!current) return current;
      const next = updated?.consent ?? {
        ...consentFor(current).find((candidate) => candidate.scope.id === scope.id)!,
        status: action === 'grant' ? 'granted' : 'revoked',
      };
      const scopes = consentFor(current).map((candidate) => candidate.scope.id === scope.id ? next : candidate);
      return { ...current, consent_scopes: scopes };
    });
  };

  if (error) return <main className="p-8"><h1>Signal unavailable</h1><p>This Signal could not be loaded.</p></main>;
  if (!detail) return <main className="p-8" aria-busy="true"><h1>Loading Signal…</h1></main>;
  const { signal } = detail;
  const consents = consentFor(detail);
  return (
    <main className="mx-auto max-w-3xl p-6 md:p-8">
      <p className="font-mono text-xs uppercase tracking-wide text-neon-cyan">Signal</p>
      <h1 className="mt-1">{signal.title}</h1>
      <p className="text-foreground-secondary">From {signal.sender.name}</p>
      {signal.body && <p className="mt-6 whitespace-pre-wrap">{signal.body}</p>}
      {signal.target?.title && <p className="mt-4 text-sm text-foreground-secondary">About: {signal.target.title}</p>}
      <section aria-labelledby="signal-consent" className="mt-8">
        <h2 id="signal-consent">Your delivery choices</h2>
        <p className="text-foreground-secondary">A Signal can be viewed without granting delivery consent. Local or fixture data does not prove that a delivery provider is active.</p>
        <div className="mt-4 grid gap-4">
          {consents.map((consent) => <ConsentControl key={consent.scope.id} consent={consent} onChange={changeConsent} />)}
          {consents.length === 0 && <p>No delivery consent is available for this Signal.</p>}
        </div>
      </section>
    </main>
  );
}
