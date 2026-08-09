import type { PortableRecord } from '../../types/touring';

export function PortableRecordBadge({ record }: { record?: PortableRecord }) {
  if (!record?.at_uri) return null;
  return <div className="mt-5 flex max-w-3xl flex-wrap items-center gap-3 border-l-2 border-neon-green pl-4 font-mono text-xs">
    <span className="uppercase text-neon-green">{record.projection_status === 'projected' ? 'Indexed from PDS' : record.projection_status?.replaceAll('_', ' ')}</span>
    {record.publisher_handle && <span className="text-foreground-secondary">@{record.publisher_handle}</span>}
    <span className="break-all text-foreground-muted">{record.at_uri}</span>
  </div>;
}
