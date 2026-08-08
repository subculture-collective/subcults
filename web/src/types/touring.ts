/**
 * Public touring DTOs. Location is deliberately limited to the server-approved
 * occurrence projection; clients must not infer coordinates from other fields.
 */
export interface PublicOccurrence {
  coarse_geohash: string;
  display_point?: { lat: number; lng: number };
  precision: 'coarse' | 'precise';
}

export interface AppearanceSummary {
  id: string;
  event: {
    id: string;
    title: string;
    starts_at: string;
    kind: string;
    occurrence: PublicOccurrence;
  };
  act: { id: string; name: string; home_territory?: string };
  tour?: { id: string; title: string };
  host_names: string[];
  context: 'tour_stop' | 'festival_appearance' | 'one_off';
  locality: 'local' | 'visiting' | 'unknown';
  status: 'announced' | 'confirmed' | 'cancelled' | 'completed';
  verification: 'unverified' | 'claimed' | 'verified' | 'disputed' | 'rejected';
}

export interface TouringProfile {
  id: string;
  name: string;
  home_territory?: string;
}

export interface TouringDetailResponse {
  profile?: TouringProfile;
  tour?: { id: string; title: string };
  appearances: AppearanceSummary[];
}
