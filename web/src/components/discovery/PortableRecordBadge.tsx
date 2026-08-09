import type { PortableRecord } from '../../types/touring';

export function PortableRecordBadge({ record }: { record?: PortableRecord }) {
  if (!record?.at_uri) return null;
  return <details className="mt-5 max-w-3xl border-l-2 border-neon-green pl-4 font-mono text-xs"><summary className="cursor-pointer uppercase text-neon-green">{record.projection_status === 'projected' ? 'Published and independently portable' : 'Publication details'}</summary><div className="mt-3 grid gap-2">{record.publisher_handle && <span className="text-foreground-secondary">Published by @{record.publisher_handle}</span>}<span className="break-all text-foreground-muted">{record.at_uri}</span></div></details>;
}
