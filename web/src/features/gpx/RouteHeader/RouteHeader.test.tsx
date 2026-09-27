// TDD contract for RouteHeader (issue 157).

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { RouteHeader } from './RouteHeader';
import type { NormalizedAnalysis, NormalizedInnerTrack } from '../normalize';

function makeTrack(overrides: Partial<NormalizedInnerTrack> = {}): NormalizedInnerTrack {
  return {
    id: 'a'.repeat(32),
    user_id: 'b'.repeat(32),
    name: 'Circular del Torrico',
    file_hash: 'abc',
    file_size_bytes: 102400,
    points: [],
    track_type: 'circular',
    uploaded_at: new Date('2025-09-15T08:00:00Z'),
    ...overrides,
  };
}

function makeAnalysis(overrides: Partial<NormalizedAnalysis> = {}): NormalizedAnalysis {
  return {
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
    ...overrides,
  };
}

describe('RouteHeader', () => {
  it('renders the track name as the main heading', () => {
    render(
      <MemoryRouter>
        <RouteHeader track={makeTrack({ name: 'Subida al Torrico' })} analysis={makeAnalysis()} />
      </MemoryRouter>,
    );
    expect(
      screen.getByRole('heading', { name: 'Subida al Torrico', level: 1 }),
    ).toBeInTheDocument();
  });

  it('renders the track type as a Chip', () => {
    render(
      <MemoryRouter>
        <RouteHeader
          track={makeTrack({ track_type: 'point-to-point' })}
          analysis={makeAnalysis()}
        />
      </MemoryRouter>,
    );
    expect(screen.getByText('point-to-point')).toBeInTheDocument();
  });

  it('formats the file size for the second chip', () => {
    render(
      <MemoryRouter>
        <RouteHeader track={makeTrack({ file_size_bytes: 2048 })} analysis={makeAnalysis()} />
      </MemoryRouter>,
    );
    expect(screen.getByText(/2\.0 kB/)).toBeInTheDocument();
  });

  it('renders the difficulty badge with the score', () => {
    render(
      <MemoryRouter>
        <RouteHeader
          track={makeTrack()}
          analysis={makeAnalysis({ difficulty_score: 87, difficulty_label: 'extreme' })}
        />
      </MemoryRouter>,
    );
    // DifficultyBadge renders the level + score
    expect(screen.getByText(/87/)).toBeInTheDocument();
  });

  it('renders a link back to the lab', () => {
    render(
      <MemoryRouter>
        <RouteHeader track={makeTrack()} analysis={makeAnalysis()} />
      </MemoryRouter>,
    );
    const back = screen.getByRole('link', { name: /laboratorio/i });
    expect(back).toHaveAttribute('href', '/lab');
  });

  it('renders the honest disclaimer about derived metrics', () => {
    render(
      <MemoryRouter>
        <RouteHeader track={makeTrack()} analysis={makeAnalysis()} />
      </MemoryRouter>,
    );
    expect(screen.getByText(/estimaciones orientativas, no diagnósticos/i)).toBeInTheDocument();
  });
});
