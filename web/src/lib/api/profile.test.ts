// TDD contract for the profile/preferences API client (phase 1.5, T2 — S3/S5).
//
// The AI consent flag travels inside the existing preferences payload:
// an explicit `ai_enabled: true` is the consent act, and an omitted flag
// is sent as `false` (omission is never consent). The rest of the
// profile is sent unchanged.
//
// No network access: fetch is stubbed.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { getPreferences, patchPreferences } from './profile';
import { ApiError } from './types';

function jsonResponse(body: unknown, status = 200): Response {
  return {
    ok: status >= 200 && status < 300,
    status,
    statusText: status === 200 ? 'OK' : 'Error',
    headers: new Headers({ 'content-type': 'application/json' }),
    json: async () => body,
  } as unknown as Response;
}

const STORED = {
  hr_max: 180,
  lthr: 150,
  ftp: 280,
  level: 'advanced' as const,
  timezone: 'Europe/Madrid',
  ai_enabled: false,
};

function bodyOf(call: unknown[]): Record<string, unknown> {
  const init = call[1] as { body?: string };
  return JSON.parse(init.body ?? '{}') as Record<string, unknown>;
}

describe('profile API client', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.stubGlobal('fetch', vi.fn());
  });

  it('fetches the preferences with the bearer token', async () => {
    vi.mocked(fetch).mockResolvedValue(jsonResponse(STORED));
    const data = await getPreferences('token-123');
    expect(data).toEqual(STORED);
    const [url, init] = vi.mocked(fetch).mock.calls[0];
    expect(url).toBe('/api/v1/me/preferences');
    expect(init?.headers).toMatchObject({ Authorization: 'Bearer token-123' });
  });

  it('sends ai_enabled=false when the caller does not provide the consent flag', async () => {
    vi.mocked(fetch).mockResolvedValue(jsonResponse(STORED));
    await patchPreferences('token-123', { timezone: 'UTC' });
    const body = bodyOf(vi.mocked(fetch).mock.calls[0]);
    expect(body.ai_enabled).toBe(false);
  });

  it('sends ai_enabled=true when the caller explicitly consents', async () => {
    vi.mocked(fetch).mockResolvedValue(jsonResponse({ ...STORED, ai_enabled: true }));
    await patchPreferences('token-123', { timezone: 'UTC', ai_enabled: true });
    const body = bodyOf(vi.mocked(fetch).mock.calls[0]);
    expect(body.ai_enabled).toBe(true);
  });

  it('keeps the rest of the profile unchanged when the consent changes', async () => {
    vi.mocked(fetch).mockResolvedValue(jsonResponse(STORED));
    await patchPreferences('token-123', {
      hr_max: 180,
      lthr: null,
      ftp: null,
      level: 'beginner',
      timezone: 'Europe/Madrid',
      ai_enabled: true,
    });
    const body = bodyOf(vi.mocked(fetch).mock.calls[0]);
    expect(body).toEqual({
      hr_max: 180,
      lthr: null,
      ftp: null,
      level: 'beginner',
      timezone: 'Europe/Madrid',
      ai_enabled: true,
    });
  });

  it('maps a 422 problem detail to field errors', async () => {
    vi.mocked(fetch).mockResolvedValue(
      jsonResponse(
        {
          type: 'https://example.com/probs/validation',
          title: 'Unprocessable Entity',
          status: 422,
          detail: 'request validation failed',
          errors: { timezone: 'timezone is required and must be a non-empty IANA name' },
        },
        422,
      ),
    );
    await expect(patchPreferences('token-123', { timezone: '' })).rejects.toBeInstanceOf(ApiError);
    await expect(patchPreferences('token-123', { timezone: '' })).rejects.toMatchObject({
      status: 422,
      fields: { timezone: 'timezone is required and must be a non-empty IANA name' },
    });
  });
});
