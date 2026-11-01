// TDD contract for DashboardSummaryCard (Fase 1.4, DW-005).
// Four metric tiles plus the weekly sparkline; with no activities the
// card shows the empty state instead of zeros.

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import type { DashboardSummary } from '../../../lib/api/dashboard';
import { DashboardSummaryCard } from './DashboardSummaryCard';

const SUMMARY: DashboardSummary = {
  weekly_volume_m: 12345.6,
  weekly_elevation_m: 320.4,
  weekly_activities_count: 4,
  trend_7d_pct: 5.2,
  trend_30d_pct: -1.5,
};

describe('DashboardSummaryCard', () => {
  it('renders the four metric tiles', () => {
    render(<DashboardSummaryCard summary={SUMMARY} sparkline={[10, 20, 30]} />);
    expect(screen.getByTestId('metric-weekly-volume')).toHaveTextContent('12346');
    expect(screen.getByTestId('metric-weekly-elevation')).toHaveTextContent('320');
    expect(screen.getByTestId('metric-weekly-activities')).toHaveTextContent('4');
    expect(screen.getByTestId('metric-trend')).toHaveTextContent('+5.2 / -1.5');
  });

  it('renders the weekly sparkline', () => {
    render(<DashboardSummaryCard summary={SUMMARY} sparkline={[10, 20, 30]} />);
    expect(screen.getByTestId('weekly-sparkline')).toBeInTheDocument();
  });

  it('renders the empty state instead of zeros when there are no activities', () => {
    render(
      <DashboardSummaryCard summary={{ ...SUMMARY, weekly_activities_count: 0 }} sparkline={[]} />,
    );
    expect(screen.getByTestId('dashboard-empty-state')).toHaveTextContent(
      'Sincroniza tus actividades de Strava para empezar',
    );
    expect(screen.queryByTestId('metric-weekly-volume')).not.toBeInTheDocument();
    expect(screen.queryByTestId('weekly-sparkline')).not.toBeInTheDocument();
  });
});
