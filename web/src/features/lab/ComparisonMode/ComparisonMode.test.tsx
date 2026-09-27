// TDD contract for ComparisonMode (issue 126).

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { ApiError } from '../../../lib/api/types';
import type { CompareResponse } from '../../../lib/api/types';

vi.mock('../RouteComparator/RouteComparator', () => ({
  RouteComparator: () => <div data-testid="route-comparator">mock</div>,
}));

vi.mock('../../../lib/api/gpx', () => ({
  compareGpxTracks: vi.fn(),
}));

import { compareGpxTracks } from '../../../lib/api/gpx';
import { ComparisonMode } from './ComparisonMode';

const mockedCompareGpxTracks = vi.mocked(compareGpxTracks);

function makeFakeTrack(
  bytes: string,
  overrides: { name: string; distance_m: number; d_plus_m: number },
) {
  // StoredTrackDetail has { track: GpxTrackSummary, climbs, risk_zones }
  // and GpxTrackSummary has { track: { id, name, ... }, analysis }.
  return {
    track: {
      track: {
        id: { bytes, valid: true },
        user_id: { bytes: 'b'.repeat(32), valid: true },
        name: overrides.name,
        file_hash: bytes,
        file_size_bytes: 100,
        points: [],
        track_type: 'circular',
        uploaded_at: '2025-09-15T08:00:00Z',
      },
      analysis: {
        distance_m: overrides.distance_m,
        moving_time_s: 3000,
        d_plus_m: overrides.d_plus_m,
        d_minus_m: 380,
        max_elevation_m: 1500,
        min_elevation_m: 300,
        elevation_coverage: null,
        avg_slope_pct: 4.2,
        max_slope_pct: 18.7,
        effort_index: 87.3,
        itra_points: 320,
        leg_breaker_index: 12.4,
        estimated_vam: 850,
        difficulty_score: 75,
        difficulty_label: 'hard' as const,
        runnability_pct: 78.5,
      },
    },
    climbs: [],
    risk_zones: [],
  };
}

const fakeResponse: CompareResponse = {
  tracks: [
    makeFakeTrack('a'.repeat(32), { name: 'A', distance_m: 12000, d_plus_m: 400 }),
    makeFakeTrack('c'.repeat(32), { name: 'B', distance_m: 13000, d_plus_m: 500 }),
  ],
  diff: {
    distance_m: {
      values: [12000, 13000],
      best_track: 0,
      unit: 'm',
    },
  },
};

beforeEach(() => {
  vi.unstubAllEnvs();
  // Use clearAllMocks (not resetAllMocks) so the mock function survives
  // between tests — each test sets its own implementation via
  // mockReturnValue / mockResolvedValue / mockRejectedValue.
  vi.clearAllMocks();
});

describe('ComparisonMode', () => {
  it('renders the loading skeleton on first render', () => {
    mockedCompareGpxTracks.mockReturnValue(new Promise(() => undefined));
    vi.stubEnv('VITE_AUTH_TOKEN', 'tok');
    render(
      <MemoryRouter>
        <ComparisonMode trackIds={['a', 'b']} />
      </MemoryRouter>,
    );
    expect(screen.getByTestId('comparison-status-loading')).toBeInTheDocument();
  });

  it('renders no-token state when VITE_AUTH_TOKEN is empty', async () => {
    vi.stubEnv('VITE_AUTH_TOKEN', '');
    render(
      <MemoryRouter>
        <ComparisonMode trackIds={['a', 'b']} />
      </MemoryRouter>,
    );
    await waitFor(() => {
      expect(screen.getByTestId('comparison-status-no-token')).toBeInTheDocument();
    });
    expect(mockedCompareGpxTracks).not.toHaveBeenCalled();
  });

  it('renders error state when fewer than 2 ids are provided', async () => {
    vi.stubEnv('VITE_AUTH_TOKEN', 'tok');
    render(
      <MemoryRouter>
        <ComparisonMode trackIds={['only-one']} />
      </MemoryRouter>,
    );
    await waitFor(() => {
      expect(screen.getByTestId('comparison-status-error')).toBeInTheDocument();
    });
    expect(mockedCompareGpxTracks).not.toHaveBeenCalled();
  });

  it('renders ready state with the fetched response', async () => {
    vi.stubEnv('VITE_AUTH_TOKEN', 'tok');
    mockedCompareGpxTracks.mockResolvedValue(fakeResponse);
    render(
      <MemoryRouter>
        <ComparisonMode trackIds={['a', 'b']} />
      </MemoryRouter>,
    );
    await waitFor(() => {
      expect(screen.getByTestId('comparison-status-ready')).toBeInTheDocument();
    });
    expect(screen.getByTestId('comparison-export-json')).toBeInTheDocument();
  });

  it('exposes the JSON export button when ready', async () => {
    vi.stubEnv('VITE_AUTH_TOKEN', 'tok');
    mockedCompareGpxTracks.mockResolvedValue(fakeResponse);
    render(
      <MemoryRouter>
        <ComparisonMode trackIds={['a', 'b']} />
      </MemoryRouter>,
    );
    await waitFor(() => {
      expect(screen.getByTestId('comparison-export-json')).toBeInTheDocument();
    });
  });

  it('renders error state when compareGpxTracks rejects', async () => {
    vi.stubEnv('VITE_AUTH_TOKEN', 'tok');
    mockedCompareGpxTracks.mockRejectedValue(new ApiError(500, 'boom'));
    render(
      <MemoryRouter>
        <ComparisonMode trackIds={['a', 'b']} />
      </MemoryRouter>,
    );
    await waitFor(() => {
      expect(screen.getByTestId('comparison-status-error')).toBeInTheDocument();
    });
    expect(screen.getByText(/500/i)).toBeInTheDocument();
  });
});
