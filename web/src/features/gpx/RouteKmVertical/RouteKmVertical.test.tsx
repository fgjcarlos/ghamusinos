// TDD contract for RouteKmVertical (issue 15, Fase 1.3).
// Singleton panel that surfaces the km_vertical result (or an
// empty-state message when the track has no qualifying ascent).

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { RouteKmVertical } from './RouteKmVertical';
import type { NormalizedKmVertical } from '../normalize';

function makeKv(overrides: Partial<NormalizedKmVertical> = {}): NormalizedKmVertical {
  return {
    start_idx: 100,
    end_idx: 350,
    gain_m: 850,
    distance_m: 10000,
    ...overrides,
  };
}

describe('RouteKmVertical', () => {
  it('renders the populated card when km_vertical is non-null', () => {
    render(<RouteKmVertical data={makeKv({ gain_m: 850, distance_m: 10000 })} />);
    expect(screen.getByTestId('route-km-vertical')).toBeInTheDocument();
    expect(screen.getByText('850')).toBeInTheDocument();
    expect(screen.getByText(/10\.00\s*km/i)).toBeInTheDocument();
    // The empty-state copy must NOT be present.
    expect(
      screen.queryByText(/este track no tiene un tramo de subida sostenida/i),
    ).not.toBeInTheDocument();
  });

  it('renders the empty-state message when km_vertical is null', () => {
    render(<RouteKmVertical data={null} />);
    expect(screen.getByTestId('route-km-vertical-empty')).toBeInTheDocument();
    expect(
      screen.getByText(/este track no tiene un tramo de subida sostenida/i),
    ).toBeInTheDocument();
    // No numerical gain should be present in the empty state.
    expect(screen.queryByText('850')).not.toBeInTheDocument();
  });
});
