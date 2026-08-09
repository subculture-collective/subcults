import type { AppearanceSummary, TouringDetailResponse } from '../types/touring';
import type { Event, Scene } from '../types/scene';
import { apiClient } from './api-client';

const API = (import.meta.env.VITE_API_URL || '/api').replace(/\/$/, '');

export async function publicRequest<T>(path: string, signal?: AbortSignal): Promise<T> {
  const response = await fetch(`${API}${path}`, { signal, credentials: 'include' });
  const body = await response.json().catch(() => ({})) as T & { data?: T; error?: { message?: string } };
  if (!response.ok) throw new Error(body.error?.message || 'The signal could not be resolved.');
  return body.data ?? body;
}

export type AppearanceFilters = {
  bbox?: string;
  from?: string;
  to?: string;
  festival?: boolean;
  kind?: string;
  locality?: 'any' | 'local' | 'visiting';
  scene?: string;
};

export async function getAppearances(filters: AppearanceFilters = {}, signal?: AbortSignal): Promise<AppearanceSummary[]> {
  const params = new URLSearchParams({ access: 'public' });
  Object.entries(filters).forEach(([key, value]) => {
    if (value !== undefined && value !== '') params.set(key, String(value));
  });
  const detail = await publicRequest<TouringDetailResponse>(`/search/appearances?${params}`, signal);
  return detail.appearances ?? [];
}

export async function setRSVP(eventID: string, status: 'going' | 'maybe'): Promise<void> {
  await apiClient.request(`/events/${encodeURIComponent(eventID)}/rsvp`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ status }), skipAutoRetry: true });
}

export async function getScene(id: string, signal?: AbortSignal): Promise<Scene> {
  return publicRequest<Scene>(`/scenes/${encodeURIComponent(id)}`, signal);
}

export async function getEvent(id: string, signal?: AbortSignal): Promise<Event & { title?: string; starts_at?: string; kind?: string; status?: string; host_names?: string[] }> {
  return publicRequest(`/events/${encodeURIComponent(id)}`, signal);
}
