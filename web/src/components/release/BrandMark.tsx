import { Link } from 'react-router-dom';

export function SignalGlyph({ className = 'h-9 w-9' }: { className?: string }) {
  return <svg className={className} viewBox="0 0 48 48" fill="none" aria-hidden="true">
    <path d="M24 3 42 13v22L24 45 6 35V13L24 3Z" stroke="currentColor" strokeWidth="2" />
    <path d="m24 10 11 7v14l-11 7-11-7V17l11-7Z" stroke="currentColor" strokeWidth="1.5" opacity=".65" />
    <circle cx="24" cy="24" r="4" fill="currentColor" />
    <path d="M24 3v17M42 13l-14 8M42 35l-14-8M24 45V28M6 35l14-8M6 13l14 8" stroke="currentColor" opacity=".35" />
  </svg>;
}

export function BrandMark({ compact = false }: { compact?: boolean }) {
  return <Link to="/" className="group flex items-center gap-3" aria-label="Subcults discovery home">
    <span className="text-neon-purple transition-colors group-hover:text-neon-cyan"><SignalGlyph /></span>
    {!compact && <span className="leading-none">
      <span className="font-display block text-2xl font-bold tracking-[.12em] text-foreground">SUBCULTS</span>
      <span className="font-mono block text-[.52rem] uppercase tracking-[.23em] text-foreground-muted">Shows, tours, scenes</span>
    </span>}
  </Link>;
}
