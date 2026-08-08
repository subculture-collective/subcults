import { Link } from 'react-router-dom';
import { useAuth } from '../stores/authStore';

export function MyActivityPage() {
  const { user, isCreator } = useAuth();
  return <main className="content-wrap py-12 lg:py-16">
    <div className="flex flex-col justify-between gap-6 border-b border-border pb-8 md:flex-row md:items-end"><div><p className="eyebrow">Participant dossier</p><h1 className="font-display mt-3 text-6xl uppercase">@{user?.handle || 'member'}</h1><p className="mt-3 text-foreground-secondary">{user?.display_name || 'Complete your identity to make this callsign yours.'}</p></div>{isCreator ? <Link to="/studio" className="button-primary">Open Studio</Link> : <Link to="/creator-access" className="button-secondary">Request Studio access</Link>}</div>
    <div className="mt-10 grid gap-6 lg:grid-cols-3">
      {[['Saved dates', 'Events, tour stops, and festival appearances you marked for later.'], ['Memberships', 'Your requested and active scene relationships.'], ['Signal consent', 'Every sender, purpose, channel, and revocation in one ledger.']].map(([title, copy]) => <section className="panel p-6" key={title}><p className="eyebrow">{title}</p><p className="mt-4 leading-7 text-foreground-secondary">{copy}</p><button className="button-quiet mt-6 border border-border">Review</button></section>)}
    </div>
  </main>;
}
