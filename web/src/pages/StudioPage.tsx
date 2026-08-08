import { Link } from 'react-router-dom';

const modules = [
  ['Scenes', 'Community identity, membership boundary, palette, and public context.'],
  ['Profiles', 'Artists, venues, festivals, collectives, labels, and promoters.'],
  ['Events', 'Occurrences, venue disclosure, lineup, access, and lifecycle.'],
  ['Tours', 'Ordered appearances without flattening festivals or one-offs.'],
  ['Imports', 'CSV provenance, reconciliation, conflicts, and corrections.'],
  ['Signals', 'Draft, audience scope, consent, schedule, delivery, and attribution.'],
];

export function StudioPage() {
  return <main className="min-h-[80vh] bg-[#0d0d15]">
    <header className="border-b border-border bg-surface/70"><div className="content-wrap flex flex-col justify-between gap-5 py-10 md:flex-row md:items-end"><div><p className="eyebrow">Creator operations</p><h1 className="font-display mt-2 text-5xl uppercase">Studio control surface</h1></div><button className="button-primary">Create new</button></div></header>
    <div className="content-wrap grid gap-8 py-10 lg:grid-cols-[240px_1fr]">
      <aside className="panel h-fit p-3"><nav className="grid" aria-label="Studio modules">{['Overview', ...modules.map(([name]) => name), 'Audience', 'Settings'].map((name, index) => <Link key={name} to={index === 0 ? '/studio' : `/studio/${name.toLowerCase()}`} className={`font-mono border-l-2 px-4 py-3 text-xs uppercase tracking-wider ${index === 0 ? 'border-neon-green bg-background-hover text-neon-green' : 'border-transparent text-foreground-muted hover:bg-background-hover hover:text-foreground'}`}>{name}</Link>)}</nav></aside>
      <section><div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">{modules.map(([name, copy], index) => <article className="panel p-6" key={name}><div className="flex justify-between"><p className="eyebrow">0{index + 1} // Module</p><span className="status-chip">Ready</span></div><h2 className="font-display mt-6 text-3xl uppercase">{name}</h2><p className="mt-3 min-h-20 leading-6 text-foreground-secondary">{copy}</p><Link to={`/studio/${name.toLowerCase()}`} className="button-secondary mt-5 w-full">Open {name}</Link></article>)}</div></section>
    </div>
  </main>;
}
