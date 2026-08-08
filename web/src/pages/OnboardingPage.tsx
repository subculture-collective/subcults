import { useState } from 'react';
import type { FormEvent } from 'react';
import { useNavigate } from 'react-router-dom';
import { completeProfile } from '../lib/auth-service';

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
  return <main className="content-wrap grid min-h-[70vh] place-items-center py-12">
    <section className="panel w-full max-w-xl p-8"><p className="eyebrow">Callsign setup</p><h1 className="font-display mt-3 text-5xl uppercase">Name your signal</h1>
      <form className="mt-8 grid gap-5" onSubmit={submit}>
        <label>Display name<input className="field mt-2" required maxLength={80} value={displayName} onChange={(event) => setDisplayName(event.target.value)} /></label>
        <label>Handle<input className="field mt-2" required minLength={3} maxLength={32} pattern="[a-zA-Z0-9_-]+" value={handle} onChange={(event) => setHandle(event.target.value.toLowerCase())} /></label>
        {error && <p className="text-status-error" role="alert">{error}</p>}
        <button className="button-primary">Complete identity</button>
      </form>
    </section>
  </main>;
}
