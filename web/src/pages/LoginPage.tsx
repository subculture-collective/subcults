import { useState } from 'react';
import type { FormEvent } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { requestMagicLink } from '../lib/auth-service';
import { BrandMark } from '../components/release/BrandMark';

export function LoginPage() {
  const location = useLocation();
  const returnPath = (location.state as { from?: { pathname?: string } } | null)?.from?.pathname || '/me';
  const [email, setEmail] = useState('');
  const [state, setState] = useState<'idle' | 'sending' | 'sent' | 'error'>('idle');
  const [message, setMessage] = useState('');
  async function submit(event: FormEvent) {
    event.preventDefault();
    setState('sending');
    try {
      await requestMagicLink(email, returnPath);
      setState('sent');
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'The access link could not be sent.');
      setState('error');
    }
  }
  return <div className="signal-grid min-h-[calc(100vh-76px)] py-14">
    <div className="content-wrap grid items-center gap-12 lg:grid-cols-[1fr_440px]">
      <div className="hidden lg:block">
        <p className="eyebrow">Authenticated frequency</p>
        <h1 className="font-display mt-5 max-w-3xl text-7xl font-bold uppercase leading-[.88]">One link.<br/><span className="text-neon-purple">No password.</span><br/>Your scenes persist.</h1>
        <p className="mt-8 max-w-xl text-lg leading-8 text-foreground-secondary">Enter to RSVP, follow artists across cities, manage consent, and request access to the creator Studio.</p>
      </div>
      <section className="panel panel-cut p-7 sm:p-9" aria-labelledby="login-heading">
        <BrandMark />
        <p className="eyebrow mt-10">Access gate</p>
        <h2 id="login-heading" className="font-display mt-3 text-4xl uppercase">Enter SUBCULT</h2>
        {state === 'sent' ? <div className="mt-8" role="status">
          <span className="status-chip" style={{ '--chip-color': 'var(--color-neon-green)' } as React.CSSProperties}>Link transmitted</span>
          <p className="mt-5 leading-7 text-foreground-secondary">If that address can receive mail, a one-time link is on its way. It expires in 15 minutes.</p>
          <button className="button-secondary mt-6" onClick={() => setState('idle')}>Use another address</button>
        </div> : <form className="mt-8" onSubmit={submit}>
          <label htmlFor="email" className="text-sm font-medium">Email address</label>
          <input id="email" className="field mt-2" type="email" autoComplete="email" required value={email} onChange={(event) => setEmail(event.target.value)} placeholder="you@scene.net" />
          {state === 'error' && <p className="mt-3 text-sm text-status-error" role="alert">{message}</p>}
          <button className="button-primary mt-5 w-full" disabled={state === 'sending'}>{state === 'sending' ? 'Transmitting…' : 'Send access link'}</button>
        </form>}
        <p className="font-mono mt-8 text-[.62rem] leading-5 text-foreground-muted">By entering, you accept the <Link className="text-neon-cyan" to="/terms">terms</Link> and <Link className="text-neon-cyan" to="/privacy">privacy contract</Link>. Access links are single-use; emails are encrypted at rest.</p>
      </section>
    </div>
  </div>;
}
