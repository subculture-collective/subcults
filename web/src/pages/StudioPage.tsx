import { useEffect, useState, type FormEvent } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { apiClient } from '../lib/api-client';
import { useAuth } from '../stores/authStore';
import { atprotoService, type PublicationResult } from '../lib/atproto-service';
import { ApiClientError } from '../lib/api-client';

const modules = [
  ['scenes', 'Scenes', 'Community identity and publication context.'],
  ['profiles', 'Profiles', 'Artists, venues, festivals, labels, and promoters.'],
  ['places', 'Places', 'Public city or region context used by occurrence discovery.'],
  ['venues', 'Venues', 'Named locations with a disclosure-safe public record.'],
  ['events', 'Events', 'Time, host scene, coarse discovery, and protected venue policy.'],
  ['tours', 'Tours', 'Act-led itineraries; appearances remain independent events.'],
  ['appearances', 'Appearances', 'Connect an act to a show, festival, tour stop, or one-off.'],
  ['signals', 'Signals', 'Draft a consent-scoped invitation or announcement.'],
] as const;

function field(form: FormData, name: string) { return String(form.get(name) || '').trim(); }
function tags(value: string) { return value.split(',').map((item) => item.trim()).filter(Boolean); }
type DraftRecord = { entityType: string; entityId: string };

export function StudioPage() {
  const location = useLocation();
  const auth = useAuth();
  const module = location.pathname.split('/')[2] || '';
  const [result, setResult] = useState<string>('');
  const [error, setError] = useState<string>('');
	const [drafts, setDrafts] = useState<DraftRecord[]>([]);
	const [publications, setPublications] = useState<PublicationResult[]>([]);
	const [publicationState, setPublicationState] = useState<'saved' | 'publishing' | 'awaiting' | 'indexed' | 'failed' | 'conflict'>('saved');
	useEffect(() => {
		if (publications.length === 0 || publicationState !== 'awaiting') return;
		let attempts = 0; const timer = window.setInterval(async () => {
			attempts += 1;
			try {
				const current = await Promise.all(publications.map((item) => atprotoService.projection(item.at_uri)));
				if (current.every((item) => item.projection_status === 'projected')) { setPublicationState('indexed'); window.clearInterval(timer); }
			}
			catch { /* authoritative write remains valid while indexing catches up */ }
			if (attempts >= 8) window.clearInterval(timer);
		}, 2000);
		return () => window.clearInterval(timer);
	}, [publications, publicationState]);
  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault(); setError(''); setResult('');
    const form = new FormData(event.currentTarget);
    try {
      let payload: Record<string, unknown>;
      if (module === 'scenes') payload = { name: field(form, 'name'), description: field(form, 'description'), owner_did: auth.user?.did, coarse_geohash: field(form, 'coarse_geohash'), visibility: field(form, 'visibility'), tags: tags(field(form, 'tags')), allow_precise: false };
      else if (module === 'profiles') payload = { canonical_name: field(form, 'canonical_name'), kind: field(form, 'kind'), visibility: field(form, 'visibility') };
		else if (module === 'places') payload = { canonical_name: field(form, 'canonical_name'), admin_region: field(form, 'admin_region') || undefined, country_code: field(form, 'country_code').toUpperCase(), timezone: field(form, 'timezone'), coarse_geohash: field(form, 'coarse_geohash') };
		else if (module === 'venues') payload = { place_id: field(form, 'place_id'), canonical_name: field(form, 'canonical_name'), coarse_geohash: field(form, 'coarse_geohash'), allow_precise: false };
		else if (module === 'events') payload = { scene_id: field(form, 'scene_id'), place_id: field(form, 'place_id'), venue_id: field(form, 'venue_id') || undefined, title: field(form, 'title'), description: field(form, 'description'), coarse_geohash: field(form, 'coarse_geohash'), starts_at: new Date(field(form, 'starts_at')).toISOString(), kind: field(form, 'kind'), location_access: field(form, 'location_access'), allow_precise: false };
      else if (module === 'tours') payload = { primary_act_id: field(form, 'primary_act_id'), title: field(form, 'title'), status: 'draft' };
      else if (module === 'appearances') payload = { event_id: field(form, 'event_id'), act_id: field(form, 'act_id'), tour_id: field(form, 'tour_id') || undefined, role: field(form, 'role'), status: 'announced' };
      else payload = { id: crypto.randomUUID(), owner_type: field(form, 'owner_type'), owner_id: field(form, 'owner_id'), target_type: field(form, 'target_type'), target_id: field(form, 'target_id'), subject: field(form, 'subject'), body: field(form, 'body'), deep_link: field(form, 'deep_link'), audience_definition: { mode: 'consented_scope' }, consent_scope_ids: tags(field(form, 'consent_scope_ids')) };
      const response = await apiClient.request<Record<string, unknown>>(`/studio/${module}`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload), skipAutoRetry: true });
		const entityType = ({ scenes: 'scene', profiles: 'profile', places: 'place', venues: 'venue', events: 'event', tours: 'tour', appearances: 'appearance' } as Record<string, string>)[module];
		const nested = module === 'profiles' ? response.profile as Record<string, unknown> : response;
		const entityId = typeof nested?.id === 'string' ? nested.id : '';
		const nextDrafts = entityType && entityId ? [{ entityType, entityId }] : [];
		const act = response.act as Record<string, unknown> | undefined;
		if (module === 'profiles' && typeof act?.id === 'string') nextDrafts.push({ entityType: 'act', entityId: act.id });
		setResult(JSON.stringify(response, null, 2));
		setDrafts(nextDrafts); setPublications([]); setPublicationState('saved'); event.currentTarget.reset();
    } catch (caught) { setError(caught instanceof Error ? caught.message : 'The draft could not be saved.'); }
  };
	const publish = async () => {
		if (drafts.length === 0) return; setError(''); setPublicationState('publishing');
		try {
			const published: PublicationResult[] = [];
			for (const draft of drafts) published.push(await atprotoService.publish(draft.entityType, draft.entityId));
			setPublications(published);
			setPublicationState(published.every((item) => item.projection_status === 'projected') ? 'indexed' : 'awaiting');
		}
		catch (caught) {
			if (caught instanceof ApiClientError && caught.status === 409) setPublicationState('conflict'); else setPublicationState('failed');
			setError(caught instanceof Error ? caught.message : 'Publication failed.');
		}
	};
  return <main className="min-h-[80vh] bg-[#0d0d15]">
    <header className="border-b border-border bg-surface/70"><div className="content-wrap py-10"><p className="eyebrow">Creator operations</p><h1 className="font-display mt-2 text-5xl uppercase">Studio control surface</h1><p className="mt-3 max-w-2xl text-foreground-secondary">Author structured records without collapsing scenes, acts, events, appearances, and tours into one object.</p></div></header>
    <div className="content-wrap grid gap-8 py-10 lg:grid-cols-[240px_1fr]">
      <aside className="panel h-fit p-3"><nav className="grid" aria-label="Studio modules"><Link to="/studio" className={navClass(module === '')}>Overview</Link>{modules.map(([slug, name]) => <Link key={slug} to={`/studio/${slug}`} className={navClass(module === slug)}>{name}</Link>)}</nav></aside>
      {!module ? <section><div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">{modules.map(([slug, name, copy], index) => <article className="panel p-6" key={slug}><p className="eyebrow">0{index + 1} // Record</p><h2 className="font-display mt-6 text-3xl uppercase">{name}</h2><p className="mt-3 min-h-20 leading-6 text-foreground-secondary">{copy}</p><Link to={`/studio/${slug}`} className="button-secondary mt-5 w-full">Create {name}</Link></article>)}</div></section> :
		<section><p className="eyebrow">New record</p><h2 className="font-display mt-2 text-5xl uppercase">{modules.find(([slug]) => slug === module)?.[1] ?? 'Studio'}</h2><form onSubmit={submit} className="panel mt-6 grid gap-5 p-6 md:grid-cols-2">{moduleFields(module)}<div className="md:col-span-2"><button className="button-primary" type="submit">Save private draft</button></div></form>{error && <p className="panel mt-4 border-danger p-4 text-danger" role="alert">{error}</p>}{drafts.length > 0 && <PublicationPanel drafts={drafts} publications={publications} state={publicationState} onPublish={publish} onUpgrade={() => atprotoService.upgrade('/studio').then(({ redirect_url }) => window.location.assign(redirect_url))}/>} {result && <details className="panel mt-4 p-5"><summary className="cursor-pointer font-mono text-xs uppercase text-foreground-muted">Draft response</summary><pre className="mt-4 overflow-auto text-xs text-neon-green" aria-live="polite">{result}</pre></details>}</section>}
    </div>
  </main>;
}

function navClass(active: boolean) { return `border-l-2 px-4 py-3 font-mono text-xs uppercase tracking-wider ${active ? 'border-neon-green bg-background-hover text-neon-green' : 'border-transparent text-foreground-muted hover:bg-background-hover hover:text-foreground'}`; }
const Input = ({ name, label, type = 'text', required = true }: { name: string; label: string; type?: string; required?: boolean }) => <label className="grid gap-2 text-sm text-foreground-secondary">{label}<input className="field" name={name} type={type} required={required} /></label>;
const Select = ({ name, label, options }: { name: string; label: string; options: string[] }) => <label className="grid gap-2 text-sm text-foreground-secondary">{label}<select className="field" name={name}>{options.map((option) => <option key={option} value={option}>{option.replaceAll('_', ' ')}</option>)}</select></label>;

function moduleFields(module: string) {
  if (module === 'scenes') return <><Input name="name" label="Scene name"/><Input name="coarse_geohash" label="Coarse geohash"/><Select name="visibility" label="Visibility" options={['public', 'private', 'unlisted']}/><Input name="tags" label="Tags, comma separated" required={false}/><label className="grid gap-2 text-sm text-foreground-secondary md:col-span-2">Description<textarea className="field min-h-32" name="description"/></label></>;
  if (module === 'profiles') return <><Input name="canonical_name" label="Public name"/><Select name="kind" label="Profile kind" options={['artist', 'venue', 'festival', 'promoter', 'collective', 'label', 'curator']}/><Select name="visibility" label="Visibility" options={['public', 'unlisted', 'private']}/></>;
	if (module === 'places') return <><Input name="canonical_name" label="Place name"/><Input name="admin_region" label="State or region" required={false}/><Input name="country_code" label="Two-letter country code"/><Input name="timezone" label="IANA timezone (for example America/Chicago)"/><Input name="coarse_geohash" label="Coarse geohash"/></>;
	if (module === 'venues') return <><Input name="canonical_name" label="Venue name"/><Input name="place_id" label="Place ID"/><Input name="coarse_geohash" label="Coarse geohash"/></>;
	if (module === 'events') return <><Input name="title" label="Event title"/><Input name="scene_id" label="Host scene ID"/><Input name="place_id" label="Occurrence place ID"/><Input name="venue_id" label="Venue ID (optional)" required={false}/><Input name="starts_at" label="Start date and time" type="datetime-local"/><Input name="coarse_geohash" label="Coarse geohash"/><Select name="kind" label="Event kind" options={['show', 'festival', 'party', 'meetup', 'broadcast', 'other']}/><Select name="location_access" label="Venue disclosure" options={['public', 'protected']}/><label className="grid gap-2 text-sm text-foreground-secondary md:col-span-2">Description<textarea className="field min-h-32" name="description"/></label></>;
  if (module === 'tours') return <><Input name="title" label="Tour title"/><Input name="primary_act_id" label="Primary act ID"/></>;
  if (module === 'appearances') return <><Input name="event_id" label="Event ID"/><Input name="act_id" label="Act ID"/><Input name="tour_id" label="Tour ID (optional)" required={false}/><Select name="role" label="Billing role" options={['headliner', 'support', 'performer', 'dj', 'speaker', 'host', 'other']}/></>;
  return <><Select name="owner_type" label="Sender type" options={['scene', 'profile']}/><Input name="owner_id" label="Sender ID"/><Select name="target_type" label="Target type" options={['event', 'tour', 'appearance', 'profile']}/><Input name="target_id" label="Target ID"/><Input name="subject" label="Signal title"/><Input name="deep_link" label="Deep link" required={false}/><Input name="consent_scope_ids" label="Consent scope IDs, comma separated"/><label className="grid gap-2 text-sm text-foreground-secondary md:col-span-2">Message body<textarea className="field min-h-32" name="body" required/></label></>;
}

function PublicationPanel({ drafts, publications, state, onPublish, onUpgrade }: { drafts: DraftRecord[]; publications: PublicationResult[]; state: string; onPublish: () => void; onUpgrade: () => void }) {
	const labels: Record<string, string> = { saved: 'Saved draft', publishing: 'Publishing to PDS', awaiting: 'Published, awaiting projection', indexed: 'Publicly indexed', failed: 'Publication failed', conflict: 'PDS record conflict' };
	return <section className="panel mt-5 border-l-4 border-l-neon-purple p-5" aria-live="polite"><div className="flex flex-wrap items-center justify-between gap-4"><div><p className="eyebrow">Publication state</p><h3 className="font-display mt-2 text-2xl uppercase">{labels[state] || state}</h3>{drafts.map((draft) => <p key={`${draft.entityType}:${draft.entityId}`} className="mt-2 font-mono text-xs text-foreground-muted">{draft.entityType} // {draft.entityId}</p>)}</div>{state === 'saved' || state === 'failed' ? <button className="button-primary" onClick={onPublish}>Publish to your PDS</button> : null}{state === 'failed' ? <button className="button-secondary" onClick={onUpgrade}>Review PDS permission</button> : null}</div>{publications.map((publication) => <div key={publication.at_uri} className="mt-5 grid gap-2 border-t border-border pt-4 font-mono text-xs"><a className="break-all text-neon-green underline" href={`https://pdsls.dev/${publication.at_uri}`} target="_blank" rel="noreferrer">{publication.at_uri}</a><span className="break-all text-foreground-muted">CID {publication.cid}</span></div>)}{state === 'conflict' && <p className="mt-4 text-sm text-danger">Reload the current PDS record before attempting to overwrite it.</p>}</section>;
}
