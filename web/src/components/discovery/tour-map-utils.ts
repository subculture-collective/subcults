import type { AppearanceSummary } from '../../types/touring';

/** Groups appearances by event, because an Event is the map's atomic occurrence. */
export function groupAppearancesByEvent(appearances: AppearanceSummary[]): Map<string, AppearanceSummary[]> {
  return appearances.reduce((groups, appearance) => {
    const group = groups.get(appearance.event.id) ?? [];
    group.push(appearance);
    groups.set(appearance.event.id, group);
    return groups;
  }, new Map<string, AppearanceSummary[]>());
}
