import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { apiClient } from '../lib/api-client';

interface CreatorRequest {
  id: string;
  user_id: string;
  statement: string;
  status: string;
  created_at: string;
}

export function AdminPage() {
  const client = useQueryClient();
  const [working, setWorking] = useState<string | null>(null);
  const query = useQuery({ queryKey: ['admin', 'creator-access'], queryFn: () => apiClient.request<{ requests: CreatorRequest[] }>('/admin/creator-access?status=pending') });
  const review = async (requestID: string, status: 'approved' | 'rejected') => {
    setWorking(requestID);
    try {
      await apiClient.request(`/admin/creator-access/${requestID}`, { method: 'PATCH', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ status }) });
      await client.invalidateQueries({ queryKey: ['admin', 'creator-access'] });
    } finally { setWorking(null); }
  };
  const requests = query.data?.requests ?? [];
  return <main>
    <header className="border-b border-border bg-surface/60"><div className="content-wrap py-12"><p className="eyebrow">Release operations</p><h1 className="font-display mt-3 text-6xl uppercase">Admin desk</h1><p className="mt-3 max-w-2xl text-foreground-secondary">Only controls backed by a live API appear here. Creator access decisions are auditable and reversible through role history.</p></div></header>
    <section className="content-wrap py-10" aria-labelledby="creator-queue"><div className="flex items-end justify-between"><div><p className="eyebrow">Approval boundary</p><h2 id="creator-queue" className="font-display mt-2 text-4xl uppercase">Creator requests</h2></div><span className="status-chip">{requests.length} pending</span></div>
      {query.isError && <div className="panel mt-6 border-danger p-6 text-danger">The approval queue could not be loaded.</div>}
      <div className="mt-6 grid gap-4">{requests.map((request) => <article key={request.id} className="panel p-6"><div className="flex flex-col justify-between gap-5 md:flex-row md:items-start"><div><p className="font-mono text-xs uppercase tracking-wider text-foreground-muted">User {request.user_id} · {new Date(request.created_at).toLocaleDateString()}</p><p className="mt-4 max-w-3xl whitespace-pre-wrap leading-7 text-foreground-secondary">{request.statement}</p></div><div className="flex shrink-0 gap-2"><button className="button-secondary" disabled={working === request.id} onClick={() => review(request.id, 'rejected')}>Reject</button><button className="button-primary" disabled={working === request.id} onClick={() => review(request.id, 'approved')}>Approve</button></div></div></article>)}{!query.isPending && requests.length === 0 && <div className="panel p-8 text-foreground-secondary">The creator approval queue is clear.</div>}</div>
    </section>
  </main>;
}
