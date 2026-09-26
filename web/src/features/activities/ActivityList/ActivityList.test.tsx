// TDD contract for ActivityList (issue 158).
// Pure presentational. Renders PeriodSummary at top, one ActivityRow
// per activity, and pagination controls. Calls onPageChange when the
// user navigates Prev/Next.

import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import type { Activity } from '../../../lib/api/types';
import { ActivityList } from './ActivityList';

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

describe('ActivityList', () => {
  it('renders empty state when activities is empty', () => {
    render(
      <MemoryRouter>
        <ActivityList activities={[]} page={1} hasNext={false} onPageChange={() => {}} />
      </MemoryRouter>,
    );
    expect(screen.getByText(/sin actividades/i)).toBeInTheDocument();
  });

  it('renders one row per activity', () => {
    const acts = [
      makeActivity({ name: 'A' }),
      makeActivity({ name: 'B' }),
      makeActivity({ name: 'C' }),
    ];
    render(
      <MemoryRouter>
        <ActivityList activities={acts} page={1} hasNext={false} onPageChange={() => {}} />
      </MemoryRouter>,
    );
    expect(screen.getAllByTestId('activity-row')).toHaveLength(3);
  });

  it('renders the PeriodSummary at top', () => {
    render(
      <MemoryRouter>
        <ActivityList
          activities={[makeActivity()]}
          page={1}
          hasNext={false}
          onPageChange={() => {}}
        />
      </MemoryRouter>,
    );
    expect(screen.getByText(/Últimas 4 semanas/i)).toBeInTheDocument();
  });

  it('renders Prev disabled when on page 1', () => {
    render(
      <MemoryRouter>
        <ActivityList
          activities={[makeActivity()]}
          page={1}
          hasNext={true}
          onPageChange={() => {}}
        />
      </MemoryRouter>,
    );
    expect(screen.getByRole('button', { name: /anterior/i })).toBeDisabled();
  });

  it('renders Next disabled when hasNext is false', () => {
    render(
      <MemoryRouter>
        <ActivityList
          activities={[makeActivity()]}
          page={1}
          hasNext={false}
          onPageChange={() => {}}
        />
      </MemoryRouter>,
    );
    expect(screen.getByRole('button', { name: /siguiente/i })).toBeDisabled();
  });

  it('calls onPageChange with page+1 when Next clicked', () => {
    const onPageChange = vi.fn();
    render(
      <MemoryRouter>
        <ActivityList
          activities={[makeActivity()]}
          page={1}
          hasNext={true}
          onPageChange={onPageChange}
        />
      </MemoryRouter>,
    );
    fireEvent.click(screen.getByRole('button', { name: /siguiente/i }));
    expect(onPageChange).toHaveBeenCalledWith(2);
  });

  it('calls onPageChange with page-1 when Prev clicked', () => {
    const onPageChange = vi.fn();
    render(
      <MemoryRouter>
        <ActivityList
          activities={[makeActivity()]}
          page={3}
          hasNext={false}
          onPageChange={onPageChange}
        />
      </MemoryRouter>,
    );
    fireEvent.click(screen.getByRole('button', { name: /anterior/i }));
    expect(onPageChange).toHaveBeenCalledWith(2);
  });
});
