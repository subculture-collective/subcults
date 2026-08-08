import { useState } from 'react';
import type { FormEvent } from 'react';
import { useAuth } from '../stores/authStore';
import { requestCreatorAccess } from '../lib/auth-service';

export function CreatorAccessPage() {
  const { isAuthenticated } = useAuth();
  const [submitted, setSubmitted] = useState(false);
  const [statement, setStatement] = useState('');
  const [error, setError] = useState('');
  const [sending, setSending] = useState(false);
  async function submit(event: FormEvent) {
    event.preventDefault(); setSending(true); setError('');
    try { await requestCreatorAccess(statement); setSubmitted(true); }
    catch (reason) { setError(reason instanceof Error ? reason.message : 'Request could not be recorded.'); }
    finally { setSending(false); }
  }
  return <main className="content-wrap grid gap-10 py-14 lg:grid-cols-[1fr_480px] lg:py-20">
    <div><p className="eyebrow">Studio access</p><h1 className="font-display mt-4 text-7xl font-bold uppercase leading-[.88]">Publish with context.<br/><span className="text-neon-purple">Keep the relationship.</span></h1><p className="mt-8 max-w-2xl text-lg leading-8 text-foreground-secondary">Beta publishing is approved by people. Tell us what you organize, represent, or maintain; imported claims remain reviewable and reversible.</p></div>
    <section className="panel p-7"><p className="eyebrow">Creator request</p>{submitted ? <div role="status"><h2 className="font-display mt-4 text-4xl uppercase">Request recorded</h2><p className="mt-4 text-foreground-secondary">An administrator will review the relationship before public publishing is enabled.</p></div> : <form className="mt-6 grid gap-5" onSubmit={submit}><label>What do you organize, represent, or maintain?<textarea className="field mt-2 min-h-52" required minLength={20} maxLength={2000} disabled={!isAuthenticated || sending} value={statement} onChange={(event) => setStatement(event.target.value)} /></label>{error && <p className="text-status-error" role="alert">{error}</p>}<button className="button-primary" disabled={!isAuthenticated || sending}>{isAuthenticated ? sending ? 'Recording…' : 'Submit for review' : 'Enter before requesting access'}</button></form>}</section>
  </main>;
}
