// TDD contract for the AI analysis API client (phase 1.5, T7 — S3/S4/S5/S6/S7).
//
// The client is the only door the web app uses to ask for an analysis:
// it must send the documented request bodies, read the documented response
// shapes and translate the RFC 9457 problem details into ApiError.
//
// No network access: fetch is stubbed.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import {
  fetchAIStatus,
  createAIAnalysis,
  listAIAnalyses,
  fetchAIAnalysis,
  type AIAnalysis,
} from './ai';
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

function problemResponse(status: number, body: unknown): Response {
  return {
    ok: false,
    status,
    statusText: 'Error',
    headers: new Headers({ 'content-type': 'application/problem+json' }),
    json: async () => body,
  } as unknown as Response;
}

const STATUS = {
  enabled: true,
  available: true,
  status: 'ready',
  provider: 'openai',
  consented: true,
};

const ANALYSIS: AIAnalysis = {
  id: '09000000-0000-0000-0000-000000000000',
  kind: 'activity',
  scope: 'activity:33333333-3333-3333-3333-333333333333',
  status: 'pending',
  provider: null,
  model: null,
  output: null,
  error_code: null,
  error_message: null,
  created_at: '2026-01-02T03:04:05Z',
  updated_at: '2026-01-02T03:04:05Z',
};

function callOf(index = 0): [string, RequestInit] {
  const call = vi.mocked(fetch).mock.calls[index];
  return [String(call[0]), (call[1] ?? {}) as RequestInit];
}

function bodyOf(index = 0): Record<string, unknown> {
  const [, init] = callOf(index);
  return JSON.parse(String(init.body ?? '{}')) as Record<string, unknown>;
}

describe('AI API client', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.stubGlobal('fetch', vi.fn());
  });

  it('fetches the status with the bearer token', async () => {
    vi.mocked(fetch).mockResolvedValue(jsonResponse(STATUS));
    const data = await fetchAIStatus('token-123');
    expect(data).toEqual(STATUS);
    const [url, init] = callOf();
    expect(url).toBe('/api/v1/ai/status');
    expect(init.headers).toMatchObject({ Authorization: 'Bearer token-123' });
  });

  it('reports the global state and the consent of the current user', async () => {
    vi.mocked(fetch).mockResolvedValue(
      jsonResponse({
        enabled: false,
        available: false,
        status: 'disabled',
        provider: null,
        consented: false,
      }),
    );
    const data = await fetchAIStatus('token-123');
    expect(data.enabled).toBe(false);
    expect(data.available).toBe(false);
    expect(data.status).toBe('disabled');
    expect(data.provider).toBeNull();
    expect(data.consented).toBe(false);
  });

  it('requests an activity analysis with only the activity id', async () => {
    vi.mocked(fetch).mockResolvedValue(jsonResponse(ANALYSIS, 202));
    const data = await createAIAnalysis('token-123', {
      kind: 'activity',
      activity_id: '33333333-3333-3333-3333-333333333333',
    });
    expect(data).toEqual(ANALYSIS);
    const [url, init] = callOf();
    expect(url).toBe('/api/v1/ai/analyses');
    expect(init.method).toBe('POST');
    expect(init.headers).toMatchObject({
      Authorization: 'Bearer token-123',
      'Content-Type': 'application/json',
    });
    expect(bodyOf()).toEqual({
      kind: 'activity',
      activity_id: '33333333-3333-3333-3333-333333333333',
    });
  });

  it('requests a window analysis without the activity id', async () => {
    vi.mocked(fetch).mockResolvedValue(jsonResponse(ANALYSIS, 202));
    await createAIAnalysis('token-123', {
      kind: 'load',
      window_start: '2026-01-05',
      window_days: 28,
    });
    expect(bodyOf()).toEqual({
      kind: 'load',
      window_start: '2026-01-05',
      window_days: 28,
    });
  });

  it('maps a 403 without consent to an ApiError', async () => {
    vi.mocked(fetch).mockResolvedValue(
      problemResponse(403, {
        type: 'about:blank',
        title: 'Forbidden',
        status: 403,
        detail:
          'ai consent required: accept the data transfer notice before requesting an analysis',
      }),
    );
    await expect(createAIAnalysis('token-123', { kind: 'weekly' })).rejects.toBeInstanceOf(
      ApiError,
    );
    await expect(createAIAnalysis('token-123', { kind: 'weekly' })).rejects.toMatchObject({
      status: 403,
      message: 'ai consent required: accept the data transfer notice before requesting an analysis',
    });
  });

  it('maps a 422 to an ApiError with the field errors', async () => {
    vi.mocked(fetch).mockResolvedValue(
      problemResponse(422, {
        type: 'about:blank',
        title: 'Unprocessable Entity',
        status: 422,
        detail: 'request validation failed',
        errors: { kind: 'kind must be one of activity | weekly | load' },
      }),
    );
    await expect(createAIAnalysis('token-123', { kind: 'weekly' })).rejects.toMatchObject({
      status: 422,
      fields: { kind: 'kind must be one of activity | weekly | load' },
    });
  });

  it('maps a 503 to an ApiError', async () => {
    vi.mocked(fetch).mockResolvedValue(
      problemResponse(503, {
        type: 'about:blank',
        title: 'Service Unavailable',
        status: 503,
        detail: 'ai analysis unavailable: the global AI flag is disabled',
      }),
    );
    await expect(createAIAnalysis('token-123', { kind: 'weekly' })).rejects.toMatchObject({
      status: 503,
      message: 'ai analysis unavailable: the global AI flag is disabled',
    });
  });

  it('lists the analyses of the current user', async () => {
    vi.mocked(fetch).mockResolvedValue(jsonResponse({ data: [ANALYSIS], limit: 20 }));
    const data = await listAIAnalyses('token-123');
    expect(data.data).toHaveLength(1);
    expect(data.limit).toBe(20);
    const [url, init] = callOf();
    expect(url).toBe('/api/v1/ai/analyses');
    expect(init.method).toBe('GET');
  });

  it('forwards the requested limit', async () => {
    vi.mocked(fetch).mockResolvedValue(jsonResponse({ data: [], limit: 5 }));
    await listAIAnalyses('token-123', { limit: 5 });
    const [url] = callOf();
    expect(url).toBe('/api/v1/ai/analyses?limit=5');
  });

  it('fetches one analysis by kind and scope', async () => {
    vi.mocked(fetch).mockResolvedValue(jsonResponse(ANALYSIS));
    const data = await fetchAIAnalysis(
      'token-123',
      'activity',
      'activity:33333333-3333-3333-3333-333333333333',
    );
    expect(data.id).toBe(ANALYSIS.id);
    const [url] = callOf();
    expect(url).toBe(
      '/api/v1/ai/analyses?kind=activity&scope=activity%3A33333333-3333-3333-3333-333333333333',
    );
  });

  it('maps a missing analysis to a 404 ApiError', async () => {
    vi.mocked(fetch).mockResolvedValue(
      problemResponse(404, {
        type: 'about:blank',
        title: 'Not Found',
        status: 404,
        detail: 'analysis not found',
      }),
    );
    await expect(
      fetchAIAnalysis('token-123', 'activity', 'activity:missing'),
    ).rejects.toMatchObject({ status: 404, message: 'analysis not found' });
  });

  it('exposes the safe error code and message of a failed analysis', async () => {
    vi.mocked(fetch).mockResolvedValue(
      jsonResponse({
        ...ANALYSIS,
        status: 'failed',
        error_code: 'provider_timeout',
        error_message: 'El proveedor de IA no respondió a tiempo.',
      }),
    );
    const data = await fetchAIAnalysis('token-123', 'activity', ANALYSIS.scope);
    expect(data.status).toBe('failed');
    expect(data.error_code).toBe('provider_timeout');
    expect(data.error_message).toBe('El proveedor de IA no respondió a tiempo.');
    expect(data.output).toBeNull();
  });
});
