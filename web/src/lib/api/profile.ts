// Profile + preferences client for /perfil (issue #159).

import { ApiError, type PreferencesApiResponse } from './types';

const BASE_URL = '/api/v1';

function makeAuthHeader(token: string): HeadersInit {
  return {
    Authorization: 'Bearer ' + token,
  };
}

function jsonResponse<T>(res: Response): Promise<T> {
  if (!res.ok) {
    throw new ApiError(res.status, `HTTP ${res.status}: ${res.statusText}`);
  }
  return res.json() as Promise<T>;
}

/**
 * GetPreferences fetches the authenticated user's preferences.
 * 200 → PreferencesApiResponse. Throws ApiError on non-2xx.
 */
export async function getPreferences(token: string): Promise<PreferencesApiResponse> {
  const res = await fetch(`${BASE_URL}/me/preferences`, {
    method: 'GET',
    headers: makeAuthHeader(token),
  });
  return jsonResponse<PreferencesApiResponse>(res);
}

/**
 * PatchPreferences validates + updates the authenticated user's
 * preferences. 200 → updated PreferencesApiResponse. 422 → ApiError
 * with field-level message.
 */
export async function patchPreferences(
  token: string,
  values: {
    hr_max?: number | null;
    lthr?: number | null;
    ftp?: number | null;
    level?: 'beginner' | 'intermediate' | 'advanced' | null;
    timezone?: string;
    ai_enabled?: boolean;
  },
): Promise<PreferencesApiResponse & { warning?: string }> {
  const body = JSON.stringify({
    hr_max: values.hr_max,
    lthr: values.lthr,
    ftp: values.ftp,
    level: values.level,
    timezone: values.timezone,
    ai_enabled: values.ai_enabled ?? false,
  });
  const res = await fetch(`${BASE_URL}/me/preferences`, {
    method: 'PATCH',
    headers: { ...makeAuthHeader(token), 'Content-Type': 'application/json' },
    body,
  });
  return jsonResponse<PreferencesApiResponse & { warning?: string }>(res);
}
