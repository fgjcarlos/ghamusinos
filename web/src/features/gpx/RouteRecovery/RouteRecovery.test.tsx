// TDD contract for RouteRecovery (issue 15, Fase 1.3).
// List of recovery zones (descents after a muro). Recovery zones
// have only distance_m — no gain_m or avg_slope_pct.

import { describe, it, expect } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import { RouteRecovery } from './RouteRecovery';
import type { NormalizedRecoveryZone } from '../normalize';

function makeRecovery(overrides: Partial<NormalizedRecoveryZone> = {}): NormalizedRecoveryZone {
  return {
    start_idx: 0,
    end_idx: 10,
    distance_m: 200,
    ...overrides,
  };
}

describe('RouteRecovery', () => {
  it('renders the empty state when there are no recovery zones', () => {
    render(<RouteRecovery data={[]} />);
    expect(screen.getByTestId('route-recovery-empty')).toBeInTheDocument();
    expect(screen.getByText(/no se detectaron zonas de recuperación/i)).toBeInTheDocument();
    expect(screen.queryByTestId('recovery-card')).not.toBeInTheDocument();
  });

  it('renders one card per recovery zone with distance_m only', () => {
    render(
      <RouteRecovery
        data={[makeRecovery({ distance_m: 200 }), makeRecovery({ distance_m: 400 })]}
      />,
    );
    const cards = screen.getAllByTestId('recovery-card');
    expect(cards).toHaveLength(2);
    // distance_m 200 -> 0.20 km.
    expect(within(cards[0]).getByText(/0\.20\s*km/i)).toBeInTheDocument();
    // distance_m 400 -> 0.40 km.
    expect(within(cards[1]).getByText(/0\.40\s*km/i)).toBeInTheDocument();
    // No severity / risk_type text is rendered.
    expect(screen.queryByText(/severidad/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/pendiente|t[eé]cnic|exposici[oó]n/i)).not.toBeInTheDocument();
  });
});
