// Profile + preferences client for /perfil (issue #159).

import { ApiError, type PreferencesApiResponse, type ProblemDetailShape } from './types';

const BASE_URL = '/api/v1';

function makeAuthHeader(token: string): HeadersInit {
  return {
    Authorization: 'Bearer ' + token,
  };
}

/**
 * parseProblemDetail returns a human-readable detail string for an
 * RFC 9457 problem+json response, preferring `detail` and falling
 * back to a generic message. Returns null when the body is not a
 * recognised problem detail (so the caller can keep using the
 * legacy HTTP status text).
 */
function parseProblemDetail(body: unknown): string | null {
  if (!body || typeof body !== 'object') return null;
  const p = body as ProblemDetailShape;
  if (typeof p.detail === 'string' && p.detail.length > 0) return p.detail;
  if (typeof p.title === 'string' && p.title.length > 0) return p.title;
  return null;
}

/**
 * jsonResponse resolves with the parsed JSON body on 2xx, or throws
 * an ApiError populated from the response status and (for problem+json
 * bodies) the field-level `errors` extension member
 * (closes-159-strava-disconnect, PR1).
 */
async function jsonResponse<T>(res: Response): Promise<T> {
  if (res.ok) {
    return res.json() as Promise<T>;
  }
  let message = `HTTP ${res.status}: ${res.statusText}`;
  let fields: Record<string, string> | undefined;
  const contentType = res.headers.get('content-type') ?? '';
  if (
    contentType.includes('application/problem+json') ||
    contentType.includes('application/json')
  ) {
    try {
      const body = await res.json();
      const detail = parseProblemDetail(body);
      if (detail) message = detail;
      if (body && typeof body === 'object') {
        const errs = (body as ProblemDetailShape).errors;
        if (errs && typeof errs === 'object') {
          fields = errs;
        }
      }
    } catch {
      // body was not JSON — keep the legacy message.
    }
  }
  throw new ApiError(res.status, message, fields);
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
