// TDD contract for PeriodSummary (issue 158).
// Calculates totals over the loaded page of activities. Honest
// disclosure: text says "de lo mostrado en esta página".

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import type { Activity } from '../../../lib/api/types';
import { PeriodSummary } from './PeriodSummary';

function makeActivity(overrides: Partial<Activity> = {}): Activity {
  return {
    id: { bytes: '0'.repeat(32), valid: true },
    user_id: { bytes: '0'.repeat(32), valid: true },
    external_source: 'strava',
    external_id: 1,
    name: 'Run',
    sport_type: 'Run',
    started_at: '2025-01-15T08:00:00Z',
    elapsed_seconds: 3600,
    moving_seconds: 3500,
    distance_meters: null,
    elevation_gain_m: null,
    avg_hr: null,
    max_hr: null,
    avg_power: null,
    raw_payload: null,
    created_at: '2025-01-15T08:00:00Z',
    updated_at: '2025-01-15T08:00:00Z',
    ...overrides,
  };
}

function makePgNumeric(value: number) {
  // Encode as bigint + exp such that bigint * 10^(-exp) * sign === value
  // For simplicity we use value as integer bigint with exp=0 and sign=1.
  return {
    bigint: String(value),
    exp: 0,
    sign: 1,
    valid: true,
  };
}

describe('PeriodSummary', () => {
  it('renders the number of activities shown in the page', () => {
    render(<PeriodSummary activities={[makeActivity(), makeActivity(), makeActivity()]} />);
    expect(screen.getByText('3')).toBeInTheDocument();
    expect(screen.getByText('Actividades')).toBeInTheDocument();
  });

  it('renders the total distance when distances are present', () => {
    const acts = [
      makeActivity({ distance_meters: makePgNumeric(5000) }),
      makeActivity({ distance_meters: makePgNumeric(8000) }),
    ];
    render(<PeriodSummary activities={acts} />);
    // 5000 + 8000 = 13000 m = 13.0 km
    expect(screen.getByText('13.0 km')).toBeInTheDocument();
  });

  it('skips activities without distance when summing', () => {
    const acts = [
      makeActivity({ distance_meters: makePgNumeric(5000) }),
      makeActivity({ distance_meters: null }),
      makeActivity({ distance_meters: makePgNumeric(2000) }),
    ];
    render(<PeriodSummary activities={acts} />);
    // 5000 + 2000 = 7000 m = 7.0 km
    expect(screen.getByText('7.0 km')).toBeInTheDocument();
  });

  it('renders the honest disclosure "de lo mostrado en esta página"', () => {
    render(<PeriodSummary activities={[]} />);
    expect(screen.getByText('de lo mostrado en esta página')).toBeInTheDocument();
  });

  it('hides the distance row entirely when no activities have distance', () => {
    const acts = [makeActivity({ distance_meters: null }), makeActivity({ distance_meters: null })];
    render(<PeriodSummary activities={acts} />);
    expect(screen.queryByText('km')).toBeNull();
  });
});
