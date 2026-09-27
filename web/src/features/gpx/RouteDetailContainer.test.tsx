// TDD contract for RouteDetailContainer (issue 157).
// TDD: tests drive the state machine. getGpxTrack is mocked so we can
// exercise loading / no-token / 404 / 500 / ready without a backend.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { ApiError } from '../../lib/api/types';
import type { GpxTrackSummary, StoredTrackDetail } from '../../lib/api/types';

vi.mock('../../lib/api/gpx', () => ({
  getGpxTrack: vi.fn(),
}));

// import after the mock so the mocked getGpxTrack is bound.
import { getGpxTrack } from '../../lib/api/gpx';
import { RouteDetailContainer } from './RouteDetailContainer';

const mockedGetGpxTrack = vi.mocked(getGpxTrack);

function makeDetail(overrides: Partial<GpxTrackSummary['track']> = {}): StoredTrackDetail {
  return {
    track: {
      track: {
        id: { bytes: '0'.repeat(32), valid: true },
        user_id: { bytes: '1'.repeat(32), valid: true },
        name: 'Circular del Torrico',
        file_hash: 'abc123',
        file_size_bytes: 102400,
        points: [],
        track_type: 'circular',
        uploaded_at: '2025-09-15T08:00:00Z',
        ...overrides,
      },
      analysis: {
        distance_m: 12345,
        moving_time_s: 3600,
        d_plus_m: 500,
        d_minus_m: 480,
        max_elevation_m: 1500,
        min_elevation_m: 300,
        avg_slope_pct: 4.2,
        max_slope_pct: 18.7,
        effort_index: 87.3,
        itra_points: 320,
        leg_breaker_index: 12.4,
        estimated_vam: 850,
        difficulty_score: 75,
        difficulty_label: 'hard',
        runnability_pct: 78.5,
      },
    },
    climbs: [],
    risk_zones: [],
  };
}

beforeEach(() => {
  vi.unstubAllEnvs();
  vi.resetAllMocks();
});

describe('RouteDetailContainer', () => {
  it('renders the loading skeleton on first render', () => {
    mockedGetGpxTrack.mockReturnValue(new Promise(() => undefined)); // never resolves
    vi.stubEnv('VITE_AUTH_TOKEN', 'tok');
    render(
      <MemoryRouter>
        <RouteDetailContainer trackId="track-1" />
      </MemoryRouter>,
    );
    expect(screen.getByTestId('route-detail-status-loading')).toBeInTheDocument();
  });

  it('renders no-token state when VITE_AUTH_TOKEN is empty', async () => {
    vi.stubEnv('VITE_AUTH_TOKEN', '');
    render(
      <MemoryRouter>
        <RouteDetailContainer trackId="track-1" />
      </MemoryRouter>,
    );
    await waitFor(() => {
      expect(screen.getByTestId('route-detail-status-no-token')).toBeInTheDocument();
    });
    expect(mockedGetGpxTrack).not.toHaveBeenCalled();
  });

  it('renders 404 state when the backend returns 404', async () => {
    vi.stubEnv('VITE_AUTH_TOKEN', 'tok');
    mockedGetGpxTrack.mockRejectedValue(new ApiError(404, 'not found'));
    render(
      <MemoryRouter>
        <RouteDetailContainer trackId="missing" />
      </MemoryRouter>,
    );
    await waitFor(() => {
      expect(screen.getByTestId('route-detail-status-not-found')).toBeInTheDocument();
    });
    expect(screen.getByText(/no existe o no es tuya/i)).toBeInTheDocument();
  });

  it('renders error state on 500', async () => {
    vi.stubEnv('VITE_AUTH_TOKEN', 'tok');
    mockedGetGpxTrack.mockRejectedValue(new ApiError(500, 'boom'));
    render(
      <MemoryRouter>
        <RouteDetailContainer trackId="track-1" />
      </MemoryRouter>,
    );
    await waitFor(() => {
      expect(screen.getByTestId('route-detail-status-error')).toBeInTheDocument();
    });
    expect(screen.getByText(/500/i)).toBeInTheDocument();
  });

  it('renders ready state with normalized data on success', async () => {
    vi.stubEnv('VITE_AUTH_TOKEN', 'tok');
    mockedGetGpxTrack.mockResolvedValue(makeDetail({ name: 'Subida al Torrico' }));
    render(
      <MemoryRouter>
        <RouteDetailContainer trackId="track-1" />
      </MemoryRouter>,
    );
    await waitFor(() => {
      expect(screen.getByTestId('route-detail-status-ready')).toBeInTheDocument();
    });
    // The container's contract is "produce a ready section whose
    // children include a rendered RouteDetail". The presentational
    // sub-components are tested separately in their own test files;
    // here we just assert the container emitted the ready section
    // and the detail artifact (which carries data-testid='route-detail').
    expect(screen.getByTestId('route-detail')).toBeInTheDocument();
  });
});
