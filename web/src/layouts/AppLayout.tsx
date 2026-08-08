import { useState } from 'react';
import { Link, NavLink, Outlet } from 'react-router-dom';
import { BrandMark } from '../components/release/BrandMark';
import { useAuth } from '../stores/authStore';

const publicNav = [
  ['/', 'Discover'],
  ['/events', 'Events'],
  ['/scenes', 'Scenes'],
  ['/search?type=profiles', 'Artists'],
  ['/search?type=tours', 'Tours'],
] as const;

function navClass({ isActive }: { isActive: boolean }) {
  return `font-mono border-b px-1 py-2 text-[.68rem] font-bold uppercase tracking-[.14em] transition-colors ${isActive ? 'border-neon-cyan text-neon-cyan' : 'border-transparent text-foreground-muted hover:text-foreground'}`;
}

export function AppLayout() {
  const { user, isAuthenticated, isCreator, logout } = useAuth();
  const [menuOpen, setMenuOpen] = useState(false);
  return <div className="min-h-screen">
    <a href="#main-content" className="button-primary fixed left-3 top-3 z-[100] -translate-y-24 focus:translate-y-0">Skip to content</a>
    <header className="sticky top-0 z-50 border-b border-border bg-background/90 backdrop-blur-xl">
      <div className="content-wrap flex h-[76px] items-center justify-between gap-6">
        <BrandMark />
        <nav aria-label="Primary navigation" className="hidden items-center gap-5 lg:flex">
          {publicNav.map(([href, label]) => <NavLink key={href} to={href} className={navClass}>{label}</NavLink>)}
        </nav>
        <div className="flex items-center gap-2">
          <Link to="/search" className="button-quiet hidden sm:inline-flex" aria-label="Search SUBCULT">Search <span aria-hidden="true">⌕</span></Link>
          {isAuthenticated ? <>
            {isCreator && <Link to="/studio" className="button-secondary hidden sm:inline-flex">Studio</Link>}
            <Link to="/me" className="button-quiet max-w-32 truncate">@{user?.handle || 'member'}</Link>
            <button className="button-quiet hidden md:inline-flex" onClick={() => void logout()}>Exit</button>
          </> : <Link to="/login" className="button-primary">Enter</Link>}
          <button className="button-quiet lg:hidden" onClick={() => setMenuOpen((open) => !open)} aria-expanded={menuOpen} aria-controls="mobile-menu">Menu</button>
        </div>
      </div>
      {menuOpen && <nav id="mobile-menu" className="content-wrap grid gap-1 border-t border-border py-3 lg:hidden" aria-label="Mobile navigation">
        {publicNav.map(([href, label]) => <NavLink key={href} to={href} onClick={() => setMenuOpen(false)} className={navClass}>{label}</NavLink>)}
      </nav>}
    </header>
    <main id="main-content"><Outlet /></main>
    <footer className="border-t border-border py-10">
      <div className="content-wrap flex flex-col justify-between gap-5 text-sm text-foreground-muted md:flex-row">
        <p className="font-mono text-xs uppercase tracking-wider">Independent scenes. Portable identity. Consented signal.</p>
        <div className="flex gap-5"><a href="/privacy">Privacy</a><a href="/terms">Terms</a><a href="mailto:info@subcult.tv">Contact</a></div>
      </div>
    </footer>
    <nav className="fixed inset-x-0 bottom-0 z-40 grid grid-cols-4 border-t border-border bg-background/95 px-2 py-2 backdrop-blur lg:hidden" aria-label="Mobile shortcuts">
      <NavLink to="/" className={navClass}>Map</NavLink>
      <NavLink to="/events" className={navClass}>Dates</NavLink>
      <NavLink to="/search" className={navClass}>Search</NavLink>
      <NavLink to={isAuthenticated ? '/me' : '/login'} className={navClass}>{isAuthenticated ? 'You' : 'Enter'}</NavLink>
    </nav>
    <div className="h-16 lg:hidden" aria-hidden="true" />
  </div>;
}
