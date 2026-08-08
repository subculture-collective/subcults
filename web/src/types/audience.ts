/**
 * A disclosure-scoped permission to deliver a Signal. Browser push permission,
 * an RSVP, membership, and contact verification are deliberately separate from
 * this record and never constitute an opt-in.
 */
export interface AudienceReference {
  id: string;
  name: string;
}

export interface ConsentScope {
  id: string;
  sender: AudienceReference & { type: 'profile' | 'scene' | string };
  channel: string;
  purpose: string;
  disclosure_version: string;
  frequency?: string;
  region?: string;
  tour?: AudienceReference;
  place?: AudienceReference;
}

export type ConsentStatus = 'granted' | 'revoked' | 'not_granted';
export type VerificationState = 'verified' | 'pending' | 'unverified';

export interface ConsentState {
  scope: ConsentScope;
  status: ConsentStatus;
  verification_state: VerificationState;
}

export type ConsentAction = 'grant' | 'revoke';

export interface ConsentMutationRequest {
  scope_id: string;
  action: ConsentAction;
}
