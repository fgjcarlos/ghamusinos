// TDD contract for ActivityRow (issue 158).
// Each row is a full-width <Link> to /actividades/:external_id with
// 7 columns: date, name, sport, distance, time, elevation, HR zones.

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import type { Activity } from '../../../lib/api/types';
import { ActivityRow } from './ActivityRow';

function makeActivity(overrides: Partial<Activity> = {}): Activity {
  return {
    id: { bytes: '0'.repeat(32), valid: true },
    user_id: { bytes: '0'.repeat(32), valid: true },
    external_source: 'strava',
    external_id: 99,
    name: 'Morning Run',
    sport_type: 'Run',
    started_at: '2025-01-15T08:00:00Z',
    elapsed_seconds: 3600,
    moving_seconds: 3500,
    distance_meters: { bigint: '5000', exp: 0, sign: 1, valid: true },
    elevation_gain_m: { bigint: '120', exp: 0, sign: 1, valid: true },
    avg_hr: { int16: 145, valid: true },
    max_hr: { int16: 168, valid: true },
    avg_power: { int16: 0, valid: false },
    raw_payload: null,
    created_at: '2025-01-15T08:00:00Z',
    updated_at: '2025-01-15T08:00:00Z',
    ...overrides,
  };
}

describe('ActivityRow', () => {
  it('renders the activity name', () => {
    render(
      <MemoryRouter>
        <ActivityRow activity={makeActivity({ name: 'My Long Run' })} />
      </MemoryRouter>,
    );
    expect(screen.getByText('My Long Run')).toBeInTheDocument();
  });

  it('renders the sport type', () => {
    render(
      <MemoryRouter>
        <ActivityRow activity={makeActivity({ sport_type: 'Ride' })} />
      </MemoryRouter>,
    );
    expect(screen.getByText('Ride')).toBeInTheDocument();
  });

  it('renders distance in km when present', () => {
    render(
      <MemoryRouter>
        <ActivityRow activity={makeActivity()} />
      </MemoryRouter>,
    );
    expect(screen.getByText(/5\.0 km/i)).toBeInTheDocument();
  });

  it('renders an em-dash when distance is null', () => {
    render(
      <MemoryRouter>
        <ActivityRow activity={makeActivity({ distance_meters: null })} />
      </MemoryRouter>,
    );
    expect(screen.getByText('—')).toBeInTheDocument();
  });

  it('renders elevation in meters', () => {
    render(
      <MemoryRouter>
        <ActivityRow activity={makeActivity()} />
      </MemoryRouter>,
    );
    expect(screen.getByText(/120 m/)).toBeInTheDocument();
  });

  it('renders a duration in h:mm:ss or m:ss format', () => {
    render(
      <MemoryRouter>
        <ActivityRow activity={makeActivity({ moving_seconds: 58 * 60 + 23 })} />
      </MemoryRouter>,
    );
    expect(screen.getByText('58:23')).toBeInTheDocument();
  });

  it('renders hours for long durations', () => {
    render(
      <MemoryRouter>
        <ActivityRow activity={makeActivity({ moving_seconds: 2 * 3600 + 15 * 60 + 30 })} />
      </MemoryRouter>,
    );
    expect(screen.getByText('2:15:30')).toBeInTheDocument();
  });

  it('renders an em-dash when elevation is null', () => {
    render(
      <MemoryRouter>
        <ActivityRow activity={makeActivity({ elevation_gain_m: null })} />
      </MemoryRouter>,
    );
    // Distance is still rendered; the em-dash is for elevation only.
    expect(screen.getByText('—')).toBeInTheDocument();
  });

  it('links to /actividades/:external_id', () => {
    render(
      <MemoryRouter>
        <ActivityRow activity={makeActivity({ external_id: 4242 })} />
      </MemoryRouter>,
    );
    const link = screen.getByRole('link');
    expect(link).toHaveAttribute('href', '/actividades/4242');
  });

  it('renders HRZoneBars without zones when not provided', () => {
    render(
      <MemoryRouter>
        <ActivityRow activity={makeActivity()} />
      </MemoryRouter>,
    );
    expect(screen.getByText('Sin FC')).toBeInTheDocument();
  });

  it('passes provided zones to HRZoneBars', () => {
    render(
      <MemoryRouter>
        <ActivityRow activity={makeActivity()} zones={[10, 20, 30, 25, 15]} />
      </MemoryRouter>,
    );
    // 5 bars should be rendered; no "Sin FC" text
    expect(screen.queryByText('Sin FC')).toBeNull();
  });
});
