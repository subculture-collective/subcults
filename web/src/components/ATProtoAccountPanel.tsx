import { useEffect, useRef, useState, type FormEvent } from 'react';
import { atprotoService, createPDSAccount, type ATProtoStatus } from '../lib/atproto-service';

declare global {
  interface Window {
    turnstile?: { render: (element: HTMLElement, options: Record<string, unknown>) => string; reset: (id?: string) => void };
  }
}

const turnstileSiteKey = import.meta.env.VITE_TURNSTILE_SITE_KEY as string | undefined;

export function ATProtoAccountPanel() {
  const [status, setStatus] = useState<ATProtoStatus | null>(null);
  const [identifier, setIdentifier] = useState('');
  const [busy, setBusy] = useState('');
  const [error, setError] = useState('');
  const [turnstileToken, setTurnstileToken] = useState('');
  const widget = useRef<HTMLDivElement>(null);

  useEffect(() => { atprotoService.status().then(setStatus).catch(() => setStatus({ linked: false })); }, []);
  useEffect(() => {
    if (!turnstileSiteKey || !widget.current || status?.linked) return;
    const render = () => window.turnstile?.render(widget.current!, {
      sitekey: turnstileSiteKey,
      callback: (token: string) => setTurnstileToken(token),
      'expired-callback': () => setTurnstileToken(''),
      theme: 'dark',
    });
    if (window.turnstile) { render(); return; }
    const script = document.createElement('script');
    script.src = 'https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit';
    script.async = true; script.defer = true; script.addEventListener('load', render); document.head.append(script);
    return () => script.removeEventListener('load', render);
  }, [status?.linked]);

  const redirect = async (action: () => Promise<{ redirect_url: string }>, name: string) => {
    setBusy(name); setError('');
    try { window.location.assign((await action()).redirect_url); }
    catch (caught) { setError(caught instanceof Error ? caught.message : 'AT Protocol authorization failed.'); setBusy(''); }
  };

  const provision = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault(); setBusy('provision'); setError('');
    const form = new FormData(event.currentTarget);
    const handle = String(form.get('handle') || '').trim();
    const email = String(form.get('email') || '').trim();
    const password = String(form.get('password') || '');
    try {
      const invitation = await atprotoService.provision(handle, turnstileToken);
	  await createPDSAccount(invitation, email, password);
      (event.currentTarget.elements.namedItem('password') as HTMLInputElement).value = '';
      await redirect(() => atprotoService.start(invitation.handle, '/me'), 'link');
    } catch (caught) {
      (event.currentTarget.elements.namedItem('password') as HTMLInputElement).value = '';
      setError(caught instanceof Error ? caught.message : 'Account provisioning failed.'); setBusy(''); window.turnstile?.reset(); setTurnstileToken('');
    }
  };

  if (status === null) return <section className="panel p-6" aria-busy="true"><p className="eyebrow">Portable publishing account</p><p className="mt-4 text-foreground-secondary">Checking account connection…</p></section>;
  if (status.linked) return <section className="panel p-6">
    <p className="eyebrow">Portable publishing account</p><div className="mt-5 flex flex-wrap items-start justify-between gap-5"><div><h2 className="font-display text-3xl uppercase">@{status.handle || status.did}</h2><p className="mt-3 text-sm text-foreground-secondary">Public Studio records will be written to the account you control.</p><details className="mt-3"><summary className="cursor-pointer font-mono text-xs uppercase text-foreground-muted">Technical account details</summary><p className="mt-2 break-all font-mono text-xs text-foreground-muted">{status.did}</p><p className="mt-1 break-all font-mono text-xs text-foreground-muted">{status.host_url}</p></details></div><span className="border border-neon-green px-3 py-1 font-mono text-xs uppercase text-neon-green">Connected</span></div>
    <div className="mt-6 flex flex-wrap gap-3"><button className="button-primary" disabled={!!busy} onClick={() => redirect(() => atprotoService.upgrade('/studio'), 'upgrade')}>{busy === 'upgrade' ? 'Opening permission…' : 'Enable Studio publishing'}</button><button className="button-secondary" disabled={!!busy} onClick={async () => { setBusy('unlink'); await atprotoService.unlink(); setStatus({ linked: false }); setBusy(''); }}>Unlink</button></div>
    {error && <p className="mt-4 text-danger" role="alert">{error}</p>}
  </section>;

  return <section className="panel p-6">
    <p className="eyebrow">Portable publishing account</p><h2 className="font-display mt-3 text-3xl uppercase">Connect an AT Protocol account</h2><p className="mt-3 max-w-2xl leading-7 text-foreground-secondary">Your email remains the recovery path for Subcults. Connecting an AT Protocol account lets you publish public records that remain portable and under your control.</p>
    <form className="mt-6 flex flex-col gap-3 sm:flex-row" onSubmit={(event) => { event.preventDefault(); redirect(() => atprotoService.start(identifier, '/me'), 'link'); }}><label className="sr-only" htmlFor="atproto-handle">AT Protocol handle or identifier</label><input id="atproto-handle" className="field flex-1" placeholder="your-handle.example.com" value={identifier} onChange={(event) => setIdentifier(event.target.value)} required/><button className="button-primary" disabled={!!busy}>{busy === 'link' ? 'Opening account…' : 'Connect account'}</button></form><details className="mt-3"><summary className="cursor-pointer text-xs text-foreground-muted">Using a DID instead of a handle?</summary><p className="mt-2 text-xs text-foreground-muted">You can paste a full AT Protocol DID in the same field.</p></details>
    <div className="my-8 border-t border-border" />
    <p className="eyebrow">Subcults-hosted option</p><h3 className="font-display mt-3 text-2xl uppercase">Create a *.subcult.tv account</h3><p className="mt-2 text-sm text-foreground-secondary">Your password goes directly from this browser to the account host. Subcults' API never receives or stores it.</p>
    {turnstileSiteKey ? <form className="mt-5 grid gap-4 md:grid-cols-2" onSubmit={provision}><label className="grid gap-2 text-sm text-foreground-secondary">Handle<input className="field" name="handle" placeholder="night-shift" minLength={3} required/></label><label className="grid gap-2 text-sm text-foreground-secondary">Email<input className="field" name="email" type="email" autoComplete="email" required/></label><label className="grid gap-2 text-sm text-foreground-secondary md:col-span-2">Account password<input className="field" name="password" type="password" autoComplete="new-password" minLength={8} required/></label><div ref={widget} className="md:col-span-2"/><div className="md:col-span-2"><button className="button-secondary" disabled={!!busy || !turnstileToken}>{busy === 'provision' ? 'Creating account…' : 'Create and connect account'}</button></div></form> : <p className="mt-5 border border-border p-4 text-sm text-foreground-muted">New Subcults-hosted accounts are not enabled here yet. You can still connect an existing AT Protocol account.</p>}
    {error && <p className="mt-4 text-danger" role="alert">{error}</p>}
  </section>;
}
