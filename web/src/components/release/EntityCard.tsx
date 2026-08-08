import { Link } from 'react-router-dom';

interface EntityCardProps {
  href: string;
  eyebrow: string;
  title: string;
  description?: string;
  meta?: string[];
  status?: string;
  accent?: 'purple' | 'cyan' | 'green' | 'orange';
}

const accents = {
  purple: 'border-l-neon-purple',
  cyan: 'border-l-neon-cyan',
  green: 'border-l-neon-green',
  orange: 'border-l-status-warning',
};

export function EntityCard({ href, eyebrow, title, description, meta = [], status, accent = 'purple' }: EntityCardProps) {
  return <Link to={href} className={`panel group block border-l-2 p-5 transition-transform hover:-translate-y-0.5 hover:border-border-hover ${accents[accent]}`}>
    <div className="flex items-start justify-between gap-4">
      <span className="eyebrow">{eyebrow}</span>
      {status && <span className="status-chip" style={{ '--chip-color': status.toLowerCase().includes('cancel') ? 'var(--color-status-error)' : 'var(--color-neon-green)' } as React.CSSProperties}>{status}</span>}
    </div>
    <h3 className="font-display mt-5 text-3xl font-semibold uppercase leading-none tracking-wide text-foreground group-hover:text-neon-cyan">{title}</h3>
    {description && <p className="mt-3 line-clamp-3 text-sm leading-6 text-foreground-secondary">{description}</p>}
    {meta.length > 0 && <div className="font-mono mt-6 flex flex-wrap gap-x-4 gap-y-2 text-[.65rem] uppercase tracking-wider text-foreground-muted">
      {meta.map((item) => <span key={item}>// {item}</span>)}
    </div>}
  </Link>;
}
