import { Link } from 'react-router-dom';
import { PageMeta } from '../components/PageMeta';

export function NotFoundPage() {
  return <main className="signal-grid grid min-h-[70vh] place-items-center px-6 py-16 text-center"><PageMeta title="Page not found"/><div><p className="eyebrow">404 // Page not found</p><h1 className="font-display mt-4 text-6xl font-bold uppercase sm:text-8xl">Nothing is listed here.</h1><p className="mx-auto mt-6 max-w-xl text-lg leading-8 text-foreground-secondary">The page may have moved, been removed, or never been public.</p><div className="mt-8 flex flex-wrap justify-center gap-3"><Link className="button-primary" to="/">Discover shows</Link><Link className="button-secondary" to="/search">Search Subcult</Link></div></div></main>;
}
