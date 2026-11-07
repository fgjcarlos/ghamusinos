/**
 * Native fetch-based client for the phase 1.6 race-planning endpoints.
 *
 * The browser never calls Open-Meteo or Overpass directly: the SPA CSP is
 * `connect-src 'self'`, so weather and terrain come from authenticated Go
 * endpoints. This client is the only door the lab uses.
 *
 * Failures are raised as `ApiError`, with the RFC 9457 `detail` used as the
 * message when the backend sends a problem+json body.
 */

import { ApiError, type ProblemDetailShape } from './types';

const BASE_URL = '/api/v1';

export type WeatherRole = 'start' | 'highest' | 'midpoint' | 'end';

export interface TrackWeatherPoint {
  role: WeatherRole;
  km: number;
  lat: number;
  lon: number;
  ele: number | null;
  time: string;
  temperature_c: number;
  wind_kmh: number;
  weather_code: number;
  label: string;
  available: boolean;
}

export interface TrackWeatherResponse {
  track_id: string;
  at: string;
  timezone: string;
  points: TrackWeatherPoint[];
}

export interface TerrainSegment {
  start_km: number;
  end_km: number;
  surface: string;
  technicality: number;
  name: string;
}

export interface TrackTerrainResponse {
  track_id: string;
  available: boolean;
  segments: TerrainSegment[];
}

/**
 * Raw Strava streams. `time` is an array of second offsets from the activity
 * start; `latlng` is an array of `[lat, lon]` pairs; `altitude` is metres.
 * Other stream types are kept as `unknown` because the lab does not read
 * them.
 */
export interface ActivityStreamsResponse {
  activity_id: string;
  started_at: string;
  streams: Record<string, unknown>;
}

export interface ActivityStreamsRefreshResponse {
  status: string;
  job_id: string;
  queued_at: string;
}

function makeAuthHeader(token: string): HeadersInit {
  return { Authorization: `Bearer ${token}` };
}

function parseProblemDetail(body: unknown): string | null {
  if (!body || typeof body !== 'object') return null;
  const problem = body as ProblemDetailShape;
  if (typeof problem.detail === 'string' && problem.detail.length > 0) return problem.detail;
  if (typeof problem.title === 'string' && problem.title.length > 0) return problem.title;
  return null;
}

async function jsonResponse<T>(response: Response): Promise<T> {
  if (response.ok) {
    return response.json() as Promise<T>;
  }
  let message = `HTTP ${response.status}: ${response.statusText}`;
  const contentType = response.headers.get('content-type') ?? '';
  if (
    contentType.includes('application/problem+json') ||
    contentType.includes('application/json')
  ) {
    try {
      const body = await response.json();
      const detail = parseProblemDetail(body);
      if (detail) message = detail;
    } catch {
      // Keep the status-based message when the body is not JSON.
    }
  }
  throw new ApiError(response.status, message);
}

function getInit(token: string, signal?: AbortSignal): RequestInit {
  const init: RequestInit = { method: 'GET', headers: makeAuthHeader(token) };
  if (signal) init.signal = signal;
  return init;
}

/** Forecast at four strategic points of a stored track. */
export async function fetchTrackWeather(
  token: string,
  trackId: string,
  at: string,
  signal?: AbortSignal,
): Promise<TrackWeatherResponse> {
  const params = new URLSearchParams({ at });
  const response = await fetch(
    `${BASE_URL}/gpx/${encodeURIComponent(trackId)}/weather?${params.toString()}`,
    getInit(token, signal),
  );
  return jsonResponse<TrackWeatherResponse>(response);
}

/** OSM surface/technicality segments for a stored track. */
export async function fetchTrackTerrain(
  token: string,
  trackId: string,
  signal?: AbortSignal,
): Promise<TrackTerrainResponse> {
  const response = await fetch(
    `${BASE_URL}/gpx/${encodeURIComponent(trackId)}/terrain`,
    getInit(token, signal),
  );
  return jsonResponse<TrackTerrainResponse>(response);
}

/**
 * Every stored stream of one activity. The backend answers 409 when `time`
 * or `latlng` is missing; callers map that status to the refresh action.
 */
export async function fetchActivityStreams(
  token: string,
  activityExternalId: string,
  signal?: AbortSignal,
): Promise<ActivityStreamsResponse> {
  const response = await fetch(
    `${BASE_URL}/activities/${encodeURIComponent(activityExternalId)}/streams`,
    getInit(token, signal),
  );
  return jsonResponse<ActivityStreamsResponse>(response);
}

/** Enqueue a Strava streams re-import for one owned activity. */
export async function refreshActivityStreams(
  token: string,
  activityExternalId: string,
): Promise<ActivityStreamsRefreshResponse> {
  const response = await fetch(
    `${BASE_URL}/activities/${encodeURIComponent(activityExternalId)}/streams/refresh`,
    { method: 'POST', headers: makeAuthHeader(token) },
  );
  return jsonResponse<ActivityStreamsRefreshResponse>(response);
}
