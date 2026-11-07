// TDD contract for the phase 1.6 race-planning API client (T9).
//
// No network: fetch is stubbed. The client must send the documented URLs and
// headers, parse successful bodies, and translate problem+json failures into
// ApiError with the backend `detail` as the message.

import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  fetchActivityStreams,
  fetchTrackTerrain,
  fetchTrackWeather,
  refreshActivityStreams,
  type ActivityStreamsResponse,
  type TrackTerrainResponse,
  type TrackWeatherResponse,
} from './lab';
import { ApiError } from './types';

function ok(body: unknown): Response {
  return {
    ok: true,
    status: 200,
    statusText: 'OK',
    headers: new Headers({ 'content-type': 'application/json' }),
    json: async () => body,
  } as unknown as Response;
}

function problem(status: number, body: unknown): Response {
  return {
    ok: false,
    status,
    statusText: 'Error',
    headers: new Headers({ 'content-type': 'application/problem+json' }),
    json: async () => body,
  } as unknown as Response;
}

function fetchMock(): ReturnType<typeof vi.fn> {
  const mock = vi.fn();
  vi.stubGlobal('fetch', mock);
  return mock;
}

afterEach(() => {
  vi.unstubAllGlobals();
});

const WEATHER: TrackWeatherResponse = {
  track_id: '11111111-1111-1111-1111-111111111111',
  at: '2026-05-01T08:00:00Z',
  timezone: 'UTC',
  points: [
    {
      role: 'start',
      km: 0,
      lat: 42.1,
      lon: -8.1,
      ele: 120,
      time: '2026-05-01T08:00:00Z',
      temperature_c: 14.2,
      wind_kmh: 12,
      weather_code: 3,
      label: 'Nublado',
      available: true,
    },
  ],
};

const TERRAIN: TrackTerrainResponse = {
  track_id: '11111111-1111-1111-1111-111111111111',
  available: true,
  segments: [{ start_km: 0, end_km: 1.2, surface: 'paved', technicality: 1, name: 'Camino' }],
};

const STREAMS: ActivityStreamsResponse = {
  activity_id: '22222222-2222-2222-2222-222222222222',
  started_at: '2026-05-01T08:00:00Z',
  streams: { time: [0, 1, 2], latlng: [[42.1, -8.1]] },
};

describe('fetchTrackWeather', () => {
  it('requests the track weather URL with the encoded at and bearer token', async () => {
    const mock = fetchMock();
    mock.mockResolvedValue(ok(WEATHER));

    await expect(fetchTrackWeather('tok', 'track-1', '2026-05-01T08:00:00Z')).resolves.toEqual(
      WEATHER,
    );

    const [url, init] = mock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe('/api/v1/gpx/track-1/weather?at=2026-05-01T08%3A00%3A00Z');
    expect(init.method).toBe('GET');
    expect((init.headers as Record<string, string>).Authorization).toBe('Bearer tok');
  });

  it('forwards an AbortSignal when provided', async () => {
    const mock = fetchMock();
    mock.mockResolvedValue(ok(WEATHER));
    const controller = new AbortController();

    await fetchTrackWeather('tok', 'track-1', '2026-05-01T08:00:00Z', controller.signal);

    const [, init] = mock.mock.calls[0] as [string, RequestInit];
    expect(init.signal).toBe(controller.signal);
  });
});

describe('fetchTrackTerrain', () => {
  it('requests the track terrain URL', async () => {
    const mock = fetchMock();
    mock.mockResolvedValue(ok(TERRAIN));

    await expect(fetchTrackTerrain('tok', 'track-1')).resolves.toEqual(TERRAIN);
    expect(mock.mock.calls[0]?.[0]).toBe('/api/v1/gpx/track-1/terrain');
  });
});

describe('fetchActivityStreams', () => {
  it('returns the stored streams', async () => {
    const mock = fetchMock();
    mock.mockResolvedValue(ok(STREAMS));

    await expect(fetchActivityStreams('tok', '12345')).resolves.toEqual(STREAMS);
    expect(mock.mock.calls[0]?.[0]).toBe('/api/v1/activities/12345/streams');
  });

  it('maps a 409 problem detail to ApiError with status and detail', async () => {
    const mock = fetchMock();
    mock.mockResolvedValue(
      problem(409, {
        type: 'about:blank',
        title: 'Streams Unavailable',
        status: 409,
        detail: 'activity streams unavailable: missing time',
      }),
    );

    const error = await fetchActivityStreams('tok', '12345').catch((err: unknown) => err);
    expect(error).toBeInstanceOf(ApiError);
    expect(error).toMatchObject({
      status: 409,
      message: 'activity streams unavailable: missing time',
    });
  });

  it('falls back to the HTTP status when the error body is not JSON', async () => {
    const mock = fetchMock();
    mock.mockResolvedValue({
      ok: false,
      status: 503,
      statusText: 'Service Unavailable',
      headers: new Headers({ 'content-type': 'text/plain' }),
    } as unknown as Response);

    await expect(fetchActivityStreams('tok', '12345')).rejects.toMatchObject({
      status: 503,
      message: 'HTTP 503: Service Unavailable',
    });
  });
});

describe('refreshActivityStreams', () => {
  it('POSTs to the refresh URL and returns the queued response', async () => {
    const mock = fetchMock();
    const body = { status: 'queued', job_id: '42', queued_at: '2026-05-01T08:00:00Z' };
    mock.mockResolvedValue(ok(body));

    await expect(refreshActivityStreams('tok', '12345')).resolves.toEqual(body);

    const [url, init] = mock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe('/api/v1/activities/12345/streams/refresh');
    expect(init.method).toBe('POST');
    expect((init.headers as Record<string, string>).Authorization).toBe('Bearer tok');
  });
});
