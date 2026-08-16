import type { AppearanceSummary, TouringDetailResponse } from '../types/touring';
import type { Event, Scene } from '../types/scene';
import { apiClient } from './api-client';

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
  const detail = await apiClient.publicRequest<TouringDetailResponse>(`/search/appearances?${params}`, signal);
  return detail.appearances ?? [];
}

export async function setRSVP(eventID: string, status: 'going' | 'maybe'): Promise<void> {
  await apiClient.request(`/events/${encodeURIComponent(eventID)}/rsvp`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ status }), skipAutoRetry: true });
}

export async function getScene(id: string, signal?: AbortSignal): Promise<Scene> {
  return apiClient.publicRequest<Scene>(`/scenes/${encodeURIComponent(id)}`, signal);
}

export async function getEvent(id: string, signal?: AbortSignal): Promise<Event & { title?: string; starts_at?: string; kind?: string; status?: string; host_names?: string[] }> {
  return apiClient.publicRequest(`/events/${encodeURIComponent(id)}`, signal);
}
