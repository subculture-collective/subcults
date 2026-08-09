import { useState } from 'react';
import type { FormEvent } from 'react';
import { useNavigate } from 'react-router-dom';
import { completeProfile } from '../lib/auth-service';
import { PageMeta } from '../components/PageMeta';

export function OnboardingPage() {
  const navigate = useNavigate();
  const [handle, setHandle] = useState('');
  const [displayName, setDisplayName] = useState('');
  const [error, setError] = useState('');
  async function submit(event: FormEvent) {
    event.preventDefault();
    try { await completeProfile(handle, displayName); navigate('/me', { replace: true }); }
    catch (reason) { setError(reason instanceof Error ? reason.message : 'Profile could not be completed.'); }
  }
  return <main className="content-wrap grid min-h-[70vh] place-items-center py-12"><PageMeta title="Create your profile"/>
    <section className="panel w-full max-w-xl p-8"><p className="eyebrow">Create your profile</p><h1 className="font-display mt-3 text-5xl uppercase">Choose how you appear</h1><p className="mt-4 leading-7 text-foreground-secondary">Your display name can be the name people know. Your handle is the short, unique name used in links and mentions.</p>
      <form className="mt-8 grid gap-5" onSubmit={submit}>
        <label>Display name<input className="field mt-2" required maxLength={80} value={displayName} onChange={(event) => setDisplayName(event.target.value)} placeholder="Maya or Night Shift Collective" /></label>
        <label>Handle<input className="field mt-2" required minLength={3} maxLength={32} pattern="[a-zA-Z0-9_-]+" value={handle} onChange={(event) => setHandle(event.target.value.toLowerCase())} placeholder="night_shift" /><span className="mt-2 block text-xs text-foreground-muted">3–32 letters, numbers, underscores, or hyphens.</span></label>
        {error && <p className="text-status-error" role="alert">{error}</p>}
        <button className="button-primary">Create profile</button>
      </form>
    </section>
  </main>;
}
