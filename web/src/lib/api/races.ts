import { ApiError } from './types';
import type { WorkoutStatus } from './workout-types';

export type Priority = 'A' | 'B' | 'C';
export interface RaceInput {
  name: string;
  race_date: string;
  distance_m: number;
  elevation_gain_m: number;
  priority: Priority;
  notes: string;
}
export interface Race extends RaceInput {
  id: string;
  created_at: string;
  updated_at: string;
}
export interface RaceDetail extends Race {
  tracks: { id: string; name: string }[];
  sessions: { id: string; title: string; scheduled_on: string; status: WorkoutStatus }[];
}
export interface RaceProgress {
  orientative: boolean;
  as_of: string;
  window_start: string;
  days_remaining: number;
  weeks_remaining: number;
  past: boolean;
  sessions: Record<WorkoutStatus | 'total', number>;
  activity_count: number;
  distance_m: number;
  elevation_gain_m: number;
  target_distance_m: number;
  target_elevation_gain_m: number;
  load: { day: string; ctl: number; atl: number; tsb: number }[];
}
interface RacePage {
  data: Race[];
  total: number;
  limit: number;
  offset: number;
  has_next: boolean;
}

const base = '/api/v1/races';
const racePath = (id: string) => `${base}/${encodeURIComponent(id)}`;

async function request<T>(token: string, path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  headers.set('Authorization', `Bearer ${token}`);
  if (init.body) headers.set('Content-Type', 'application/json');
  const res = await fetch(path, { ...init, headers });
  if (!res.ok) throw new ApiError(res.status, `HTTP ${res.status}`);
  return res.status === 204 ? (undefined as T) : (res.json() as Promise<T>);
}

const options = (signal?: AbortSignal): RequestInit => (signal ? { signal } : {});
export const listRaces = (token: string, limit = 20, offset = 0, signal?: AbortSignal) =>
  request<RacePage>(token, `${base}?limit=${limit}&offset=${offset}`, options(signal));
export const getRace = (token: string, id: string, signal?: AbortSignal) =>
  request<RaceDetail>(token, racePath(id), options(signal));
export const getRaceProgress = (token: string, id: string, signal?: AbortSignal) =>
  request<RaceProgress>(token, `${racePath(id)}/progress`, options(signal));
export const saveRace = (token: string, input: RaceInput, id?: string) =>
  request<Race>(token, id ? racePath(id) : base, {
    method: id ? 'PUT' : 'POST',
    body: JSON.stringify(input),
  });
export const deleteRace = (token: string, id: string) =>
  request<void>(token, racePath(id), { method: 'DELETE' });
export const setRaceTrack = (token: string, id: string, trackId: string, attached: boolean) =>
  request<void>(token, `${racePath(id)}/tracks/${encodeURIComponent(trackId)}`, {
    method: attached ? 'PUT' : 'DELETE',
  });

export async function listRaceOptions(token: string, signal: AbortSignal): Promise<Race[]> {
  const races: Race[] = [];
  for (;;) {
    const page = await listRaces(token, 100, races.length, signal);
    races.push(...page.data);
    if (!page.has_next || !page.data.length) return races;
  }
}
