import { useState } from 'react';
import type { FormEvent } from 'react';
import { useAuth } from '../stores/authStore';
import { requestCreatorAccess } from '../lib/auth-service';
import { Link } from 'react-router-dom';
import { PageMeta } from '../components/PageMeta';

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
  return <main className="content-wrap grid gap-10 py-14 lg:grid-cols-[1fr_480px] lg:py-20"><PageMeta title="Creator Studio access"/>
    <div><p className="eyebrow">Creator Studio</p><h1 className="font-display mt-4 text-7xl font-bold uppercase leading-[.88]">Publish with context.<br/><span className="text-neon-purple">Keep the relationship.</span></h1><p className="mt-8 max-w-2xl text-lg leading-8 text-foreground-secondary">Artists, venues, promoters, festivals, labels, and scene organizers can request beta access. Tell us what you represent or maintain so public claims stay accountable.</p><p className="mt-5 text-sm text-foreground-muted">There is no fee to apply. We’ll email you when your request has been reviewed.</p></div>
    <section className="panel p-7"><p className="eyebrow">Creator request</p>{submitted ? <div role="status"><h2 className="font-display mt-4 text-4xl uppercase">Request received</h2><p className="mt-4 text-foreground-secondary">We’ll review the relationship and email you when a decision is available.</p></div> : isAuthenticated ? <form className="mt-6 grid gap-5" onSubmit={submit}><label>What do you organize, represent, or maintain?<textarea className="field mt-2 min-h-52" required minLength={20} maxLength={2000} disabled={sending} value={statement} onChange={(event) => setStatement(event.target.value)} placeholder="Tell us the artist, venue, festival, label, promoter, or scene you work with and your role." /></label>{error && <p className="text-status-error" role="alert">{error}</p>}<button className="button-primary" disabled={sending}>{sending ? 'Submitting…' : 'Submit for review'}</button></form> : <div className="mt-6"><p className="leading-7 text-foreground-secondary">Sign in first so we can connect the request to your verified email.</p><Link className="button-primary mt-5" to="/login" state={{ from: { pathname: '/creator-access' } }}>Sign in to apply</Link></div>}</section>
  </main>;
}
