import { Link } from 'react-router-dom';
import type { AppearanceSummary } from '../../types/touring';
import { formatDateTime } from '../../utils/dateTime';
import { PortableRecordBadge } from './PortableRecordBadge';

export interface AppearanceCardProps {
  appearance: AppearanceSummary;
}

const contextLabels: Record<AppearanceSummary['context'], string> = {
  tour_stop: 'Tour stop',
  festival_appearance: 'Festival appearance',
  one_off: 'One-off appearance',
};

/** A concise, public-facing occurrence card. Host and home territory are never conflated. */
export function AppearanceCard({ appearance }: AppearanceCardProps) {
  const hosts = appearance.host_names.filter(Boolean).join(', ');
  const visiting = appearance.locality === 'visiting' && appearance.act.home_territory;

  return (
    <article className="border border-border bg-background-secondary p-4 shadow-none" aria-labelledby={`appearance-${appearance.id}`}>
      <div className="mb-2 flex flex-wrap items-center gap-2 text-xs font-semibold uppercase tracking-wide text-foreground-muted">
        <span>{contextLabels[appearance.context]}</span>
        <span aria-label={`Status: ${appearance.status}`}>[{appearance.status}]</span>
      </div>
      <h3 id={`appearance-${appearance.id}`} className="m-0 text-lg font-semibold text-foreground">
        <Link className="focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-primary" to={`/events/${appearance.event.id}`}>
          {appearance.event.title}
        </Link>
      </h3>
      <p className="mt-2 mb-0 font-mono text-sm text-foreground-secondary">
        {formatDateTime(appearance.event.starts_at)}
      </p>
      <p className="mt-2 mb-0 text-sm text-foreground">{appearance.act.name}</p>
      {hosts && <p className="mt-2 mb-0 text-sm text-foreground-secondary">Hosted by {hosts}</p>}
      {visiting && <p className="mt-1 mb-0 text-sm text-foreground-secondary">Visiting from {appearance.act.home_territory}</p>}
      {appearance.locality === 'local' && <p className="mt-1 mb-0 text-sm text-foreground-secondary">Local appearance</p>}
      {appearance.tour && (
        <p className="mt-3 mb-0 text-sm text-foreground-secondary">
          <Link className="underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-primary" to={`/tours/${appearance.tour.id}`}>
            {appearance.tour.title}
          </Link>
        </p>
      )}
      <PortableRecordBadge record={appearance} />
    </article>
  );
}
