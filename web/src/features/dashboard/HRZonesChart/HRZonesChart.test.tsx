// TDD contract for HRZonesChart (Fase 1.4, DW-004).
// Stacked bar with the five buckets and the explicit "degraded" label
// when the backend had to approximate the zones.

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import type { HRZoneBucket } from '../../../lib/api/dashboard';
import { HRZonesChart } from './HRZonesChart';

const ZONES: HRZoneBucket[] = [
  { zone: 'z1', minutes: 120 },
  { zone: 'z2', minutes: 90 },
  { zone: 'z3', minutes: 60 },
  { zone: 'z4', minutes: 30 },
  { zone: 'z5', minutes: 10 },
];

describe('HRZonesChart', () => {
  it('renders the chart container', () => {
    render(<HRZonesChart zones={ZONES} degraded={false} />);
    expect(screen.getByTestId('hr-zones-chart')).toBeInTheDocument();
  });

  it('renders the five zone buckets in the legend', () => {
    render(<HRZonesChart zones={ZONES} degraded={false} />);
    const legend = screen.getByTestId('hr-zones-legend');
    expect(legend).toHaveTextContent('z1');
    expect(legend).toHaveTextContent('z2');
    expect(legend).toHaveTextContent('z3');
    expect(legend).toHaveTextContent('z4');
    expect(legend).toHaveTextContent('z5');
  });

  it('warns about the approximation when the zones are degraded', () => {
    render(<HRZonesChart zones={ZONES} degraded />);
    expect(screen.getByTestId('hr-zones-degraded-label')).toHaveTextContent(
      'aproximación (sin streams HR)',
    );
  });

  it('does not render the approximation label when the zones are exact', () => {
    render(<HRZonesChart zones={ZONES} degraded={false} />);
    expect(screen.queryByTestId('hr-zones-degraded-label')).not.toBeInTheDocument();
    expect(screen.queryByText('aproximación (sin streams HR)')).not.toBeInTheDocument();
  });
});
