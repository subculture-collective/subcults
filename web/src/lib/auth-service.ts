import { API_V1, authStore, type User } from '../stores/authStore';

interface DataEnvelope<T> { data: T }

interface AuthPayload {
  access_token: string;
  user: User;
  return_path?: string;
}

async function read<T>(response: Response): Promise<T> {
  const body = await response.json().catch(() => ({})) as DataEnvelope<T> & { error?: { message?: string } };
  if (!response.ok) throw new Error(body.error?.message || 'Request failed');
  return body.data;
}

export async function requestMagicLink(email: string, returnPath = '/'): Promise<void> {
  await read(await fetch(`${API_V1}/auth/magic-links`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    body: JSON.stringify({ email, return_path: returnPath }),
  }));
}

export async function verifyMagicLink(token: string): Promise<AuthPayload> {
  const payload = await read<AuthPayload>(await fetch(`${API_V1}/auth/magic-links/verify`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    body: JSON.stringify({ token }),
  }));
  authStore.setUser(payload.user, payload.access_token);
  return payload;
}

export async function completeProfile(handle: string, displayName: string): Promise<User> {
  const token = authStore.getState().accessToken;
  const user = await read<User>(await fetch(`${API_V1}/auth/profile`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    body: JSON.stringify({ handle, display_name: displayName }),
  }));
  if (token) authStore.setUser(user, token);
  return user;
}

export async function getCurrentUser(): Promise<User> {
  const token = authStore.getState().accessToken;
  return read<User>(await fetch(`${API_V1}/me`, { headers: { Authorization: `Bearer ${token}` } }));
}

export interface CreatorAccessRequest {
  id: string;
  statement: string;
  status: 'pending' | 'approved' | 'rejected' | 'withdrawn';
  created_at: string;
  review_note?: string;
}

export async function requestCreatorAccess(statement: string): Promise<CreatorAccessRequest> {
  const token = authStore.getState().accessToken;
  return read<CreatorAccessRequest>(await fetch(`${API_V1}/creator-access`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    body: JSON.stringify({ statement }),
  }));
}

export async function logout(): Promise<void> { await authStore.logout(); }

// Compatibility export for older callers; password credentials are no longer accepted.
export async function login(credentials: { username: string }): Promise<User> {
  await requestMagicLink(credentials.username);
  throw new Error('Check your email for a one-time access link.');
}
