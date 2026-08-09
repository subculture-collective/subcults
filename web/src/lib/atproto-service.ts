import { apiClient } from './api-client';

type Envelope<T> = { data: T } | T;
function unwrap<T>(value: Envelope<T>): T {
  return typeof value === 'object' && value !== null && 'data' in value ? value.data : value;
}

export interface ATProtoStatus {
  linked: boolean;
  did?: string;
  handle?: string;
  host_url?: string;
  status?: string;
  granted_scopes?: string[];
  linked_at?: string;
}

export interface ProvisioningResult {
  request_id: string;
  handle: string;
  invite_code: string;
  pds_url: string;
  create_endpoint: string;
  expires_at: string;
}

export interface PublicationResult {
  at_uri: string;
  cid: string;
  projection_status: 'awaiting_projection' | 'projected';
  record_version: number;
}

export const atprotoService = {
  async status() {
    return unwrap(await apiClient.request<Envelope<ATProtoStatus>>('/auth/atproto/status'));
  },
  async start(identifier: string, returnPath = '/me') {
    return unwrap(await apiClient.request<Envelope<{ redirect_url: string }>>('/auth/atproto/start', {
      method: 'POST', body: JSON.stringify({ identifier, return_path: returnPath }), skipAutoRetry: true,
    }));
  },
  async upgrade(returnPath = '/studio') {
    return unwrap(await apiClient.request<Envelope<{ redirect_url: string }>>('/auth/atproto/upgrade', {
      method: 'POST', body: JSON.stringify({ return_path: returnPath }), skipAutoRetry: true,
    }));
  },
  unlink() {
    return apiClient.request('/auth/atproto/link', { method: 'DELETE', skipAutoRetry: true });
  },
  async provision(handle: string, turnstileToken: string) {
    return unwrap(await apiClient.request<Envelope<ProvisioningResult>>('/auth/atproto/provision', {
      method: 'POST', body: JSON.stringify({ handle, turnstile_token: turnstileToken }), skipAutoRetry: true,
    }));
  },
  async publish(entityType: string, entityId: string, swapCID?: string) {
    return unwrap(await apiClient.request<Envelope<PublicationResult>>('/studio/atproto/publish', {
      method: 'POST', body: JSON.stringify({ entity_type: entityType, entity_id: entityId, swap_cid: swapCID }), skipAutoRetry: true,
    }));
  },
  async projection(uri: string) {
    return unwrap(await apiClient.request<Envelope<PublicationResult & { updated_at: string }>>(`/atproto/projections?uri=${encodeURIComponent(uri)}`));
  },
};

// createPDSAccount intentionally bypasses the Subcults API. The PDS password
// exists only in the caller's form control and this direct request body.
export async function createPDSAccount(invitation: ProvisioningResult, email: string, password: string) {
  const response = await fetch(invitation.create_endpoint, {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, handle: invitation.handle, password, inviteCode: invitation.invite_code }),
  });
  if (!response.ok) throw new Error('The PDS did not create the account. The invitation may have expired.');
}
