import { useState, type FormEvent } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { apiClient } from '../lib/api-client';
import { useAuth } from '../stores/authStore';

const modules = [
  ['scenes', 'Scenes', 'Community identity and publication context.'],
  ['profiles', 'Profiles', 'Artists, venues, festivals, labels, and promoters.'],
  ['events', 'Events', 'Time, host scene, coarse discovery, and protected venue policy.'],
  ['tours', 'Tours', 'Act-led itineraries; appearances remain independent events.'],
  ['appearances', 'Appearances', 'Connect an act to a show, festival, tour stop, or one-off.'],
  ['signals', 'Signals', 'Draft a consent-scoped invitation or announcement.'],
] as const;

function field(form: FormData, name: string) { return String(form.get(name) || '').trim(); }
function tags(value: string) { return value.split(',').map((item) => item.trim()).filter(Boolean); }

export function StudioPage() {
  const location = useLocation();
  const auth = useAuth();
  const module = location.pathname.split('/')[2] || '';
  const [result, setResult] = useState<string>('');
  const [error, setError] = useState<string>('');
  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault(); setError(''); setResult('');
    const form = new FormData(event.currentTarget);
    try {
      let payload: Record<string, unknown>;
      if (module === 'scenes') payload = { name: field(form, 'name'), description: field(form, 'description'), owner_did: auth.user?.did, coarse_geohash: field(form, 'coarse_geohash'), visibility: field(form, 'visibility'), tags: tags(field(form, 'tags')), allow_precise: false };
      else if (module === 'profiles') payload = { canonical_name: field(form, 'canonical_name'), kind: field(form, 'kind'), visibility: field(form, 'visibility') };
      else if (module === 'events') payload = { scene_id: field(form, 'scene_id'), title: field(form, 'title'), description: field(form, 'description'), coarse_geohash: field(form, 'coarse_geohash'), starts_at: new Date(field(form, 'starts_at')).toISOString(), kind: field(form, 'kind'), location_access: field(form, 'location_access'), allow_precise: false };
      else if (module === 'tours') payload = { primary_act_id: field(form, 'primary_act_id'), title: field(form, 'title'), status: 'draft' };
      else if (module === 'appearances') payload = { event_id: field(form, 'event_id'), act_id: field(form, 'act_id'), tour_id: field(form, 'tour_id') || undefined, role: field(form, 'role'), status: 'announced' };
      else payload = { id: crypto.randomUUID(), owner_type: field(form, 'owner_type'), owner_id: field(form, 'owner_id'), target_type: field(form, 'target_type'), target_id: field(form, 'target_id'), subject: field(form, 'subject'), body: field(form, 'body'), deep_link: field(form, 'deep_link'), audience_definition: { mode: 'consented_scope' }, consent_scope_ids: tags(field(form, 'consent_scope_ids')) };
      const response = await apiClient.request<Record<string, unknown>>(`/studio/${module}`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload), skipAutoRetry: true });
      setResult(JSON.stringify(response, null, 2)); event.currentTarget.reset();
    } catch (caught) { setError(caught instanceof Error ? caught.message : 'The draft could not be saved.'); }
  };
  return <main className="min-h-[80vh] bg-[#0d0d15]">
    <header className="border-b border-border bg-surface/70"><div className="content-wrap py-10"><p className="eyebrow">Creator operations</p><h1 className="font-display mt-2 text-5xl uppercase">Studio control surface</h1><p className="mt-3 max-w-2xl text-foreground-secondary">Author structured records without collapsing scenes, acts, events, appearances, and tours into one object.</p></div></header>
    <div className="content-wrap grid gap-8 py-10 lg:grid-cols-[240px_1fr]">
      <aside className="panel h-fit p-3"><nav className="grid" aria-label="Studio modules"><Link to="/studio" className={navClass(module === '')}>Overview</Link>{modules.map(([slug, name]) => <Link key={slug} to={`/studio/${slug}`} className={navClass(module === slug)}>{name}</Link>)}</nav></aside>
      {!module ? <section><div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">{modules.map(([slug, name, copy], index) => <article className="panel p-6" key={slug}><p className="eyebrow">0{index + 1} // Record</p><h2 className="font-display mt-6 text-3xl uppercase">{name}</h2><p className="mt-3 min-h-20 leading-6 text-foreground-secondary">{copy}</p><Link to={`/studio/${slug}`} className="button-secondary mt-5 w-full">Create {name}</Link></article>)}</div></section> :
      <section><p className="eyebrow">New record</p><h2 className="font-display mt-2 text-5xl uppercase">{modules.find(([slug]) => slug === module)?.[1] ?? 'Studio'}</h2><form onSubmit={submit} className="panel mt-6 grid gap-5 p-6 md:grid-cols-2">{moduleFields(module)}<div className="md:col-span-2"><button className="button-primary" type="submit">Save draft</button></div></form>{error && <p className="panel mt-4 border-danger p-4 text-danger" role="alert">{error}</p>}{result && <pre className="panel mt-4 overflow-auto p-5 text-xs text-neon-green" aria-live="polite">{result}</pre>}</section>}
    </div>
  </main>;
}

function navClass(active: boolean) { return `border-l-2 px-4 py-3 font-mono text-xs uppercase tracking-wider ${active ? 'border-neon-green bg-background-hover text-neon-green' : 'border-transparent text-foreground-muted hover:bg-background-hover hover:text-foreground'}`; }
const Input = ({ name, label, type = 'text', required = true }: { name: string; label: string; type?: string; required?: boolean }) => <label className="grid gap-2 text-sm text-foreground-secondary">{label}<input className="field" name={name} type={type} required={required} /></label>;
const Select = ({ name, label, options }: { name: string; label: string; options: string[] }) => <label className="grid gap-2 text-sm text-foreground-secondary">{label}<select className="field" name={name}>{options.map((option) => <option key={option} value={option}>{option.replaceAll('_', ' ')}</option>)}</select></label>;

function moduleFields(module: string) {
  if (module === 'scenes') return <><Input name="name" label="Scene name"/><Input name="coarse_geohash" label="Coarse geohash"/><Select name="visibility" label="Visibility" options={['public', 'private', 'unlisted']}/><Input name="tags" label="Tags, comma separated" required={false}/><label className="grid gap-2 text-sm text-foreground-secondary md:col-span-2">Description<textarea className="field min-h-32" name="description"/></label></>;
  if (module === 'profiles') return <><Input name="canonical_name" label="Public name"/><Select name="kind" label="Profile kind" options={['artist', 'venue', 'festival', 'promoter', 'collective', 'label', 'curator']}/><Select name="visibility" label="Visibility" options={['public', 'unlisted', 'private']}/></>;
  if (module === 'events') return <><Input name="title" label="Event title"/><Input name="scene_id" label="Host scene ID"/><Input name="starts_at" label="Start date and time" type="datetime-local"/><Input name="coarse_geohash" label="Coarse geohash"/><Select name="kind" label="Event kind" options={['show', 'festival', 'party', 'meetup', 'broadcast', 'other']}/><Select name="location_access" label="Venue disclosure" options={['public', 'protected']}/><label className="grid gap-2 text-sm text-foreground-secondary md:col-span-2">Description<textarea className="field min-h-32" name="description"/></label></>;
  if (module === 'tours') return <><Input name="title" label="Tour title"/><Input name="primary_act_id" label="Primary act ID"/></>;
  if (module === 'appearances') return <><Input name="event_id" label="Event ID"/><Input name="act_id" label="Act ID"/><Input name="tour_id" label="Tour ID (optional)" required={false}/><Select name="role" label="Billing role" options={['headliner', 'support', 'performer', 'dj', 'speaker', 'host', 'other']}/></>;
  return <><Select name="owner_type" label="Sender type" options={['scene', 'profile']}/><Input name="owner_id" label="Sender ID"/><Select name="target_type" label="Target type" options={['event', 'tour', 'appearance', 'profile']}/><Input name="target_id" label="Target ID"/><Input name="subject" label="Signal title"/><Input name="deep_link" label="Deep link" required={false}/><Input name="consent_scope_ids" label="Consent scope IDs, comma separated"/><label className="grid gap-2 text-sm text-foreground-secondary md:col-span-2">Message body<textarea className="field min-h-32" name="body" required/></label></>;
}
