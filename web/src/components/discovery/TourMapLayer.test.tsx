import { describe, expect, it } from 'vitest';
import { groupAppearancesByEvent } from './TourMapLayer';
import type { AppearanceSummary } from '../../types/touring';

const appearance = (id: string): AppearanceSummary => ({ id, event: { id: 'event-1', title: 'One event', starts_at: '2026-09-01T20:00:00Z', kind: 'show', occurrence: { coarse_geohash: 'dp3', display_point: { lat: 41.88, lng: -87.63 }, precision: 'coarse' } }, act: { id, name: id }, host_names: [], context: 'tour_stop', locality: 'unknown', status: 'announced', verification: 'claimed' });
describe('groupAppearancesByEvent', () => { it('groups multiple appearances for one event into one map marker group', () => { expect(groupAppearancesByEvent([appearance('a'), appearance('b')]).get('event-1')).toHaveLength(2); }); });
