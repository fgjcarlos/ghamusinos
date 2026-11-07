import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '../../../../lib/api/types';
import type {
  ActivityStreamsResponse,
  TrackTerrainResponse,
  TrackWeatherResponse,
} from '../../../../lib/api/lab';
import { useActivityStreams } from './useActivityStreams';
import { useTrackTerrain } from './useTrackTerrain';
import { useTrackWeather } from './useTrackWeather';

vi.mock('../../../../lib/api/lab', () => ({
  fetchTrackWeather: vi.fn(),
  fetchTrackTerrain: vi.fn(),
  fetchActivityStreams: vi.fn(),
  refreshActivityStreams: vi.fn(),
}));

import * as labApi from '../../../../lib/api/lab';

const fetchWeather = vi.mocked(labApi.fetchTrackWeather);
const fetchTerrain = vi.mocked(labApi.fetchTrackTerrain);
const fetchStreams = vi.mocked(labApi.fetchActivityStreams);
const refreshStreams = vi.mocked(labApi.refreshActivityStreams);

const WEATHER: TrackWeatherResponse = {
  track_id: 'track-1',
  at: '2026-05-01T08:00:00Z',
  timezone: 'UTC',
  points: [],
};

const TERRAIN: TrackTerrainResponse = { track_id: 'track-1', available: false, segments: [] };

const STREAMS: ActivityStreamsResponse = {
  activity_id: 'activity-1',
  started_at: '2026-05-01T08:00:00Z',
  streams: { time: [0, 1], latlng: [[42.1, -8.1]] },
};

beforeEach(() => {
  vi.clearAllMocks();
});

afterEach(() => {
  vi.clearAllMocks();
});

describe('useTrackWeather', () => {
  it('does not fetch until load is called', () => {
    renderHook(() => useTrackWeather('tok', 'track-1'));
    expect(fetchWeather).not.toHaveBeenCalled();
  });

  it('loads the forecast for the requested instant', async () => {
    fetchWeather.mockResolvedValue(WEATHER);
    const { result } = renderHook(() => useTrackWeather('tok', 'track-1'));

    await act(async () => {
      await result.current.load('2026-05-01T08:00:00Z');
    });

    expect(fetchWeather).toHaveBeenCalledWith('tok', 'track-1', '2026-05-01T08:00:00Z');
    expect(result.current.data).toEqual(WEATHER);
    expect(result.current.loading).toBe(false);
    expect(result.current.error).toBeNull();
  });

  it('surfaces an ApiError detail as the section error', async () => {
    fetchWeather.mockRejectedValue(new ApiError(503, 'weather forecast unavailable'));
    const { result } = renderHook(() => useTrackWeather('tok', 'track-1'));

    await act(async () => {
      await result.current.load('2026-05-01T08:00:00Z');
    });

    expect(result.current.data).toBeNull();
    expect(result.current.error).toBe('weather forecast unavailable');
  });
});

describe('useTrackTerrain', () => {
  it('loads terrain on demand', async () => {
    fetchTerrain.mockResolvedValue(TERRAIN);
    const { result } = renderHook(() => useTrackTerrain('tok', 'track-1'));

    await act(async () => {
      await result.current.load();
    });

    expect(fetchTerrain).toHaveBeenCalledWith('tok', 'track-1');
    expect(result.current.data).toEqual(TERRAIN);
  });
});

describe('useActivityStreams', () => {
  it('loads streams and reaches the ready state', async () => {
    fetchStreams.mockResolvedValue(STREAMS);
    const { result } = renderHook(() => useActivityStreams('tok', 'activity-1'));

    await act(async () => {
      await result.current.load();
    });

    expect(result.current.state).toEqual({ kind: 'ready', data: STREAMS });
  });

  it('maps a 409 to needs-refresh and enqueues the re-import', async () => {
    fetchStreams.mockRejectedValue(new ApiError(409, 'activity streams unavailable: missing time'));
    refreshStreams.mockResolvedValue({
      status: 'queued',
      job_id: '42',
      queued_at: '2026-05-01T08:00:00Z',
    });
    const { result } = renderHook(() => useActivityStreams('tok', 'activity-1'));

    await act(async () => {
      await result.current.load();
    });
    expect(result.current.state).toEqual({
      kind: 'needs-refresh',
      detail: 'activity streams unavailable: missing time',
    });

    await act(async () => {
      await result.current.refresh();
    });
    expect(refreshStreams).toHaveBeenCalledWith('tok', 'activity-1');
    expect(result.current.state.kind).toBe('needs-refresh');
  });
});
