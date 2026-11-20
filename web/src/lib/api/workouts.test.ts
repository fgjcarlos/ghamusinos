import { afterEach, describe, expect, it, vi } from 'vitest';
import { createWorkout, deleteWorkout, listWorkouts, updateWorkout } from './workouts';

const response = (status: number, body?: unknown) =>
  ({
    ok: status >= 200 && status < 300,
    status,
    json: vi.fn(async () => body),
    text: vi.fn(async () => JSON.stringify(body)),
  }) as unknown as Response;

afterEach(() => vi.unstubAllGlobals());

describe('workouts API', () => {
  it('lists with escaped inclusive civil-date parameters and bearer auth', async () => {
    const fetch = vi
      .fn()
      .mockResolvedValue(
        response(200, { data: [], total: 0, limit: 20, offset: 0, has_next: false }),
      );
    vi.stubGlobal('fetch', fetch);
    await listWorkouts('token', '2026-01-01', '2026-12-31');
    expect(fetch.mock.calls[0]?.[0]).toBe(
      '/api/v1/workouts?from=2026-01-01&to=2026-12-31&limit=20&offset=0',
    );
    expect(fetch.mock.calls[0]?.[1].headers.Authorization).toBe('Bearer token');
  });

  it('sends create bodies and exposes ProblemDetail instance', async () => {
    const fetch = vi
      .fn()
      .mockResolvedValue(response(500, { detail: 'failed', instance: 'req-123' }));
    vi.stubGlobal('fetch', fetch);
    const workout: import('./workout-types').WorkoutInput = {
      scheduled_on: '2026-02-01',
      title: 'Easy',
      workout_type: 'rodaje',
      target_duration_sec: 0,
      target_distance_m: null,
      target_elevation_gain_m: null,
      target_intensity: null,
    };
    await expect(createWorkout('token', workout)).rejects.toMatchObject({
      status: 500,
      instance: 'req-123',
    });
    expect(fetch.mock.calls[0]?.[1].method).toBe('POST');
    expect(JSON.parse(fetch.mock.calls[0]?.[1].body)).toEqual(workout);
  });

  it('patches an encoded id with nullable fields preserved in the JSON body', async () => {
    const fetch = vi.fn().mockResolvedValue(response(200, { id: 'a' }));
    vi.stubGlobal('fetch', fetch);
    const patch = { target_duration_sec: 0, target_distance_m: null };
    await updateWorkout('token', 'id/unsafe', patch);
    expect(fetch.mock.calls[0]?.[0]).toBe('/api/v1/workouts/id%2Funsafe');
    expect(fetch.mock.calls[0]?.[1].method).toBe('PATCH');
    expect(JSON.parse(fetch.mock.calls[0]?.[1].body)).toEqual(patch);
  });

  it('retains status when a problem response is not valid JSON', async () => {
    const fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 503,
      json: vi.fn().mockRejectedValue(new SyntaxError('bad JSON')),
    });
    vi.stubGlobal('fetch', fetch);
    await expect(listWorkouts('token', '2026-01-01', '2026-12-31')).rejects.toMatchObject({
      status: 503,
      message: 'HTTP 503',
    });
  });

  it('retains status for null problem JSON and network failures', async () => {
    const fetch = vi.fn().mockResolvedValue(response(500, null));
    vi.stubGlobal('fetch', fetch);
    await expect(listWorkouts('token', '2026-01-01', '2026-12-31')).rejects.toMatchObject({
      status: 500,
      message: 'HTTP 500',
    });
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('offline')));
    await expect(listWorkouts('token', '2026-01-01', '2026-12-31')).rejects.toThrow('offline');
  });

  it('deletes without parsing a 204 body', async () => {
    const result = response(204);
    const fetch = vi.fn().mockResolvedValue(result);
    vi.stubGlobal('fetch', fetch);
    await deleteWorkout('token', 'id/unsafe');
    expect(fetch.mock.calls[0]?.[0]).toBe('/api/v1/workouts/id%2Funsafe');
    expect(result.json).not.toHaveBeenCalled();
  });
});
