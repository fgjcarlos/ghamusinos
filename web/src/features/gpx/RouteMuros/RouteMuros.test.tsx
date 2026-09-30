// TDD contract for RouteMuros (issue 15, Fase 1.3).
// List of muros (short, steep climbs detected by FindMuros).

import { describe, it, expect } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import { RouteMuros } from './RouteMuros';
import type { NormalizedMuro } from '../normalize';

function makeMuro(overrides: Partial<NormalizedMuro> = {}): NormalizedMuro {
  return {
    start_idx: 0,
    end_idx: 10,
    gain_m: 100,
    distance_m: 800,
    avg_slope_pct: 12.5,
    ...overrides,
  };
}

describe('RouteMuros', () => {
  it('renders the empty state when there are no muros', () => {
    render(<RouteMuros data={[]} />);
    expect(screen.getByTestId('route-muros-empty')).toBeInTheDocument();
    expect(screen.getByText(/no se detectaron muros/i)).toBeInTheDocument();
    expect(screen.queryByTestId('muro-card')).not.toBeInTheDocument();
  });

  it('renders one card per muro with the numeric metrics', () => {
    render(<RouteMuros data={[makeMuro({ gain_m: 120, distance_m: 900, avg_slope_pct: 13.3 })]} />);
    const card = screen.getByTestId('muro-card');
    expect(card).toBeInTheDocument();
    // gain_m is the integer 120 (toFixed(0)).
    expect(within(card).getByText('120')).toBeInTheDocument();
    // distance_m is 900 -> 0.90 km.
    expect(within(card).getByText(/0\.90\s*km/i)).toBeInTheDocument();
    // avg_slope_pct is 13.3 (toFixed(1)).
    expect(within(card).getByText(/13\.3%/)).toBeInTheDocument();
  });

  it('renders one card per muro, in the same DOM order as the data array', () => {
    render(
      <RouteMuros
        data={[
          makeMuro({ start_idx: 10 }),
          makeMuro({ start_idx: 30 }),
          makeMuro({ start_idx: 50 }),
        ]}
      />,
    );
    const cards = screen.getAllByTestId('muro-card');
    expect(cards).toHaveLength(3);
    expect(within(cards[0]).getByText('10')).toBeInTheDocument();
    expect(within(cards[1]).getByText('30')).toBeInTheDocument();
    expect(within(cards[2]).getByText('50')).toBeInTheDocument();
  });
});
