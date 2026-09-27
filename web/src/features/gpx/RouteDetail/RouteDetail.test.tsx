// TDD contract for RouteDetail (issue 157).
// Composes the presentational sub-components with the normalized data.

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { RouteDetail } from './RouteDetail';
import type { NormalizedTrackDetail } from '../normalize';

function makeData(overrides: Partial<NormalizedTrackDetail> = {}): NormalizedTrackDetail {
  return {
    track: {
      id: 'a'.repeat(32),
      user_id: 'b'.repeat(32),
      name: 'Circular del Torrico',
      file_hash: 'abc',
      file_size_bytes: 102400,
      points: [],
      track_type: 'circular',
      uploaded_at: new Date('2025-09-15T08:00:00Z'),
    },
    analysis: {
      distance_m: 12345,
      moving_time_s: 3600,
      d_plus_m: 500,
      d_minus_m: 480,
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
      difficulty_label: 'hard',
      runnability_pct: 78.5,
    },
    climbs: [
      {
        id: 'c'.repeat(32),
        start_idx: 0,
        end_idx: 10,
        gain_m: 200,
        distance_m: 2000,
        avg_slope_pct: 10,
        is_king_climb: true,
        vam: 850,
      },
    ],
    risk_zones: [{ start_idx: 5, end_idx: 8, category: 'steep', severity: 0.85 }],
    ...overrides,
  };
}

describe('RouteDetail', () => {
  it('renders an article with data-testid="route-detail"', () => {
    render(
      <MemoryRouter>
        <RouteDetail data={makeData()} />
      </MemoryRouter>,
    );
    expect(screen.getByTestId('route-detail')).toBeInTheDocument();
  });

  it('renders the RouteHeader with the track name', () => {
    render(
      <MemoryRouter>
        <RouteDetail data={makeData({ track: { ...makeData().track, name: 'Custom' } })} />
      </MemoryRouter>,
    );
    expect(screen.getByRole('heading', { name: 'Custom', level: 1 })).toBeInTheDocument();
  });

  it('renders the metrics tiles', () => {
    render(
      <MemoryRouter>
        <RouteDetail data={makeData()} />
      </MemoryRouter>,
    );
    // "Distancia" appears both in the metrics tile label AND in each
    // climb's stat list — use getAllByText and assert at least one.
    expect(screen.getAllByText('Distancia').length).toBeGreaterThan(0);
    expect(screen.getByText('Desnivel +')).toBeInTheDocument();
  });

  it('renders the climb list', () => {
    render(
      <MemoryRouter>
        <RouteDetail data={makeData()} />
      </MemoryRouter>,
    );
    expect(screen.getByTestId('climb-king')).toBeInTheDocument();
  });

  it('renders the risk list', () => {
    render(
      <MemoryRouter>
        <RouteDetail data={makeData()} />
      </MemoryRouter>,
    );
    expect(screen.getAllByTestId('risk')).toHaveLength(1);
  });

  it('renders a non-degenerate elevation profile for real points', () => {
    const data = makeData({
      track: {
        ...makeData().track,
        points: [
          { lat: 40.4, lng: -3.7, ele: 1000 },
          { lat: 40.41, lng: -3.69, ele: 1010 },
          { lat: 40.42, lng: -3.68, ele: 1020 },
          { lat: 40.43, lng: -3.67, ele: 1030 },
          { lat: 40.44, lng: -3.66, ele: 1040 },
        ],
      },
    });
    render(
      <MemoryRouter>
        <RouteDetail data={data} />
      </MemoryRouter>,
    );
    const profile = screen.getByTestId('elevation-profile');
    expect(
      profile.querySelector('path')?.getAttribute('d')?.match(/L/g)?.length,
    ).toBeGreaterThanOrEqual(4);
    expect(profile.innerHTML).not.toContain('NaN');
  });

  it('renders the elevation profile svg', () => {
    render(
      <MemoryRouter>
        <RouteDetail data={makeData()} />
      </MemoryRouter>,
    );
    expect(screen.getByTestId('elevation-profile')).toBeInTheDocument();
  });
});
