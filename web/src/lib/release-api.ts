import type { AppearanceSummary, TouringDetailResponse } from '../types/touring';
import type { Event, Scene } from '../types/scene';

const API = (import.meta.env.VITE_API_URL || '/api').replace(/\/$/, '');

export async function publicRequest<T>(path: string, signal?: AbortSignal): Promise<T> {
  const response = await fetch(`${API}${path}`, { signal, credentials: 'include' });
  const body = await response.json().catch(() => ({})) as T & { data?: T; error?: { message?: string } };
  if (!response.ok) throw new Error(body.error?.message || 'The signal could not be resolved.');
  return body.data ?? body;
}

export async function getAppearances(signal?: AbortSignal): Promise<AppearanceSummary[]> {
  const detail = await publicRequest<TouringDetailResponse>('/search/appearances?access=public', signal);
  return detail.appearances ?? [];
}

export async function getScene(id: string, signal?: AbortSignal): Promise<Scene> {
  return publicRequest<Scene>(`/scenes/${encodeURIComponent(id)}`, signal);
}

export async function getEvent(id: string, signal?: AbortSignal): Promise<Event & { title?: string; starts_at?: string; kind?: string; status?: string; host_names?: string[] }> {
  return publicRequest(`/events/${encodeURIComponent(id)}`, signal);
}
