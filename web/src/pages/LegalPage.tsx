import { useLocation } from 'react-router-dom';

export function LegalPage() {
  const privacy = useLocation().pathname === '/privacy';
  return <main className="content-wrap max-w-4xl py-16"><p className="eyebrow">Public contract</p><h1 className="font-display mt-4 text-6xl uppercase">{privacy ? 'Privacy' : 'Terms'}</h1><div className="dossier-rule mt-8 space-y-6 pt-8 leading-8 text-foreground-secondary"><p>{privacy ? 'SUBCULT separates participation, identity, location disclosure, and messaging consent. Public discovery never receives protected exact locations or private contact values.' : 'The public beta is an evolving service for scene discovery and approved creator publishing. Abuse, impersonation, scraping of protected data, and unauthorized commercial messaging are prohibited.'}</p><p>The complete reviewed policy document will replace this release summary before the public-beta gate is approved.</p></div></main>;
}
