/**
 * Public touring DTOs. Location is deliberately limited to the server-approved
 * occurrence projection; clients must not infer coordinates from other fields.
 */
export interface PublicOccurrence {
  coarse_geohash: string;
  display_point?: { lat: number; lng: number };
  precision: 'coarse' | 'precise';
}

export interface PortableRecord {
  at_uri?: string;
  cid?: string;
  publisher_did?: string;
  publisher_handle?: string;
  projection_status?: 'reserved' | 'awaiting_projection' | 'projected' | 'failed' | 'conflict' | 'deleted' | 'quarantined';
}

export interface AppearanceSummary extends PortableRecord {
  id: string;
  event: {
    id: string;
    title: string;
    starts_at: string;
    kind: string;
    occurrence: PublicOccurrence;
  };
  act: { id: string; profile_id: string; name: string; home_territory?: string };
  tour?: { id: string; title: string };
  host_names: string[];
  context: 'tour_stop' | 'festival_appearance' | 'one_off';
  locality: 'local' | 'visiting' | 'unknown';
  status: 'announced' | 'confirmed' | 'cancelled' | 'completed';
  verification: 'unverified' | 'claimed' | 'verified' | 'disputed' | 'rejected';
}

export interface TouringProfile extends PortableRecord {
  id: string;
  name: string;
  home_territory?: string;
}

export interface TouringDetailResponse {
  profile?: TouringProfile;
  tour?: { id: string; title: string } & PortableRecord;
  appearances: AppearanceSummary[];
}
