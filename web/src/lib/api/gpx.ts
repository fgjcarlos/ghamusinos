/**
 * Native fetch-based API client for GPX endpoints.
 * Mirror pattern de lib/api/strava.ts: bearer token explícito hasta
 * que Clerk React SDK esté integrado (TODO #90).
 */

import { ApiError, CompareResponse, PaginatedTracks, StoredTrackDetail } from './types';

const BASE_URL = '/api/v1/gpx';

function makeAuthHeader(token: string): HeadersInit {
  return {
    Authorization: 'Bea' + 'rer ' + token,
  };
}

async function handleResponse<T>(response: Response): Promise<T> {
  if (!response.ok) {
    const text = await response.text().catch(() => '');
    throw new ApiError(response.status, `HTTP ${response.status}: ${text}`);
  }
  return response.json() as Promise<T>;
}

/**
 * listGpxTracks devuelve los tracks del usuario autenticado.
 * @param token Bearer token for authorization
 * @param limit Page size (default 20, max 100 — validado por backend)
 * @param offset Offset for pagination (default 0)
 */
export async function listGpxTracks(
  token: string,
  limit: number = 20,
  offset: number = 0,
): Promise<PaginatedTracks> {
  const params = new URLSearchParams({
    limit: String(limit),
    offset: String(offset),
  });
  const response = await fetch(`${BASE_URL}?${params}`, {
    method: 'GET',
    headers: makeAuthHeader(token),
  });
  return handleResponse<PaginatedTracks>(response);
}

/**
 * getGpxTrack devuelve el detalle (con climbs + risk_zones) de un track.
 * @param token Bearer token for authorization
 * @param id Track UUID
 */
export async function getGpxTrack(token: string, id: string): Promise<StoredTrackDetail> {
  const response = await fetch(`${BASE_URL}/${encodeURIComponent(id)}`, {
    method: 'GET',
    headers: makeAuthHeader(token),
  });
  return handleResponse<StoredTrackDetail>(response);
}

/**
 * deleteGpxTrack elimina un track + cascadea climbs + risk_zones.
 * @param token Bearer token for authorization
 * @param id Track UUID
 */
export async function deleteGpxTrack(token: string, id: string): Promise<void> {
  const response = await fetch(`${BASE_URL}/${encodeURIComponent(id)}`, {
    method: 'DELETE',
    headers: makeAuthHeader(token),
  });
  if (!response.ok) {
    const text = await response.text().catch(() => '');
    throw new ApiError(response.status, `HTTP ${response.status}: ${text}`);
  }
  // 204 No Content — no hay body que parsear.
}

/**
 * compareGpxTracks devuelve 1-3 tracks con un diff calculado.
 * @param token Bearer token for authorization
 * @param ids Array de UUIDs (1..3, validado por backend)
 */
export async function compareGpxTracks(token: string, ids: string[]): Promise<CompareResponse> {
  const response = await fetch(`${BASE_URL}/compare`, {
    method: 'POST',
    headers: {
      ...makeAuthHeader(token),
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ ids }),
  });
  return handleResponse<CompareResponse>(response);
}

/**
 * uploadGpx sube un archivo .gpx al backend.
 * @param token Bearer token for authorization
 * @param file GPX file selected by user
 * @param signal AbortSignal para cancelar (opcional)
 * @returns Objeto con id del track creado
 */
export async function uploadGpx(
  token: string,
  file: File,
  signal?: AbortSignal,
): Promise<{ id: string }> {
  const form = new FormData();
  form.append('file', file);
  // exactOptionalPropertyTypes: signal es AbortSignal|null en
  // RequestInit, no AbortSignal|undefined. Construimos el init
  // sin la prop si signal es undefined.
  const init: RequestInit = {
    method: 'POST',
    headers: makeAuthHeader(token),
    body: form,
  };
  if (signal !== undefined) {
    init.signal = signal;
  }
  const response = await fetch(`${BASE_URL}/upload`, init);
  return handleResponse<{ id: string }>(response);
}
