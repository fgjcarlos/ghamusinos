// TDD contract for TrainingLoadChart (Fase 1.4, DW-004).
// The chart container is always in the DOM and the legend is real text
// (CTL, ATL, TSB). The canvas itself is not asserted on: ECharts draws
// on a canvas, which jsdom cannot measure.

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import type { TrainingLoadPoint } from '../../../lib/api/dashboard';
import { TrainingLoadChart } from './TrainingLoadChart';

const SERIES: TrainingLoadPoint[] = [
  { day: '2026-01-13', ctl: 40, atl: 45, tsb: -5, tss: 30 },
  { day: '2026-01-14', ctl: 42, atl: 48, tsb: -6, tss: 50 },
  { day: '2026-01-15', ctl: 44, atl: 50, tsb: -6, tss: 20 },
];

describe('TrainingLoadChart', () => {
  it('renders the chart container', () => {
    render(<TrainingLoadChart series={SERIES} />);
    expect(screen.getByTestId('training-load-chart')).toBeInTheDocument();
  });

  it('renders the CTL, ATL and TSB legend as text', () => {
    render(<TrainingLoadChart series={SERIES} />);
    const legend = screen.getByTestId('training-load-legend');
    expect(legend).toHaveTextContent('CTL');
    expect(legend).toHaveTextContent('ATL');
    expect(legend).toHaveTextContent('TSB');
  });
});
