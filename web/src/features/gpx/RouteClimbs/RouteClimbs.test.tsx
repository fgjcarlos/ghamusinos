// TDD contract for RouteClimbs (issue 157).

import { describe, it, expect } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import { RouteClimbs } from './RouteClimbs';
import type { NormalizedClimb } from '../normalize';

function makeClimb(overrides: Partial<NormalizedClimb> = {}): NormalizedClimb {
  return {
    id: 'a'.repeat(32),
    start_idx: 0,
    end_idx: 10,
    gain_m: 200,
    distance_m: 2000,
    avg_slope_pct: 10,
    is_king_climb: false,
    vam: 850,
    ...overrides,
  };
}

describe('RouteClimbs', () => {
  it('renders an empty state when no climbs', () => {
    render(<RouteClimbs climbs={[]} />);
    expect(screen.getByTestId('route-climbs-empty')).toBeInTheDocument();
    expect(screen.getByText(/no se detectaron subidas/i)).toBeInTheDocument();
  });

  it('renders the count in the heading when there are climbs', () => {
    render(<RouteClimbs climbs={[makeClimb(), makeClimb()]} />);
    expect(screen.getByRole('heading', { name: /Subidas \(2\)/ })).toBeInTheDocument();
  });

  it('renders one item per climb', () => {
    render(
      <RouteClimbs
        climbs={[
          makeClimb(),
          makeClimb(),
          makeClimb({ is_king_climb: true, gain_m: 400, distance_m: 1500 }),
        ]}
      />,
    );
    // 2 normal climbs + 1 king climb
    expect(screen.getAllByTestId('climb')).toHaveLength(2);
    expect(screen.getByTestId('climb-king')).toBeInTheDocument();
  });

  it('flags the king climb with the badge label', () => {
    render(
      <RouteClimbs climbs={[makeClimb({ is_king_climb: true, gain_m: 500, distance_m: 1500 })]} />,
    );
    expect(screen.getByText('Subida reina')).toBeInTheDocument();
  });

  it('formats gain, distance and slope inside the stat dl', () => {
    render(
      <RouteClimbs climbs={[makeClimb({ gain_m: 200, distance_m: 2000, avg_slope_pct: 10 })]} />,
    );
    const item = screen.getByTestId('climb');
    expect(within(item).getByText('200 m')).toBeInTheDocument();
    expect(within(item).getByText('2.00 km')).toBeInTheDocument();
    expect(within(item).getByText('10.0%')).toBeInTheDocument();
  });

  it('renders the VAM stat only when present', () => {
    render(<RouteClimbs climbs={[makeClimb({ vam: null })]} />);
    expect(screen.queryByText(/VAM/)).toBeNull();
  });
});
