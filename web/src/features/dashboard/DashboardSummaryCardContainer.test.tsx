// TDD contract for DashboardSummaryCardContainer (Fase 1.4, DW-005).
// The container owns the data: loading, error and populated states.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import type { DashboardSummary } from '../../lib/api/dashboard';
import { ApiError } from '../../lib/api/types';
import { createDeferred } from '../../test/deferred';
import { DashboardSummaryCardContainer } from './DashboardSummaryCardContainer';

vi.mock('../../lib/api/dashboard', () => ({
  fetchDashboardSummary: vi.fn(),
  fetchTrainingLoad: vi.fn(),
  fetchHRZones: vi.fn(),
  fetchCardiacDrift: vi.fn(),
  postRecalc: vi.fn(),
}));

import * as dashboardApi from '../../lib/api/dashboard';

const mockedSummary = vi.mocked(dashboardApi.fetchDashboardSummary);
const mockedLoad = vi.mocked(dashboardApi.fetchTrainingLoad);

const SUMMARY: DashboardSummary = {
  weekly_volume_m: 1000,
  weekly_elevation_m: 100,
  weekly_activities_count: 3,
  trend_7d_pct: 1,
  trend_30d_pct: 2,
};

function renderContainer() {
  return render(<DashboardSummaryCardContainer from="2026-01-01" to="2026-01-31" />);
}

describe('DashboardSummaryCardContainer', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.stubEnv('VITE_AUTH_TOKEN', 'test-token');
    mockedLoad.mockResolvedValue({
      from: '2026-01-01',
      to: '2026-01-31',
      series: [
        { day: '2026-01-29', ctl: 40, atl: 45, tsb: -5, tss: 10 },
        { day: '2026-01-30', ctl: 41, atl: 46, tsb: -5, tss: 20 },
        { day: '2026-01-31', ctl: 42, atl: 47, tsb: -5, tss: 30 },
      ],
    });
  });

  it('renders the loading state while the summary is in flight', async () => {
    const deferred = createDeferred<DashboardSummary>();
    mockedSummary.mockReturnValue(deferred.promise);
    const { unmount } = renderContainer();
    expect(screen.getByTestId('dashboard-summary-loading')).toBeInTheDocument();
    expect(screen.queryByTestId('dashboard-summary-card')).not.toBeInTheDocument();
    // Settle the request so the shared in-flight map is cleaned up.
    unmount();
    deferred.resolve(SUMMARY);
    await deferred.promise;
  });

  it('renders the error state when the summary fails', async () => {
    mockedSummary.mockRejectedValue(new ApiError(500, 'boom'));
    renderContainer();
    const error = await screen.findByTestId('dashboard-summary-error');
    expect(error).toHaveTextContent('Error 500: boom');
    expect(screen.queryByTestId('dashboard-summary-card')).not.toBeInTheDocument();
  });

  it('renders the card with the tiles and the sparkline', async () => {
    mockedSummary.mockResolvedValue(SUMMARY);
    renderContainer();
    expect(await screen.findByTestId('dashboard-summary-card')).toBeInTheDocument();
    expect(screen.getByTestId('metric-weekly-volume')).toHaveTextContent('1000');
    expect(screen.getByTestId('weekly-sparkline')).toBeInTheDocument();
  });

  it('renders the empty state when the week has no activities', async () => {
    mockedSummary.mockResolvedValue({ ...SUMMARY, weekly_activities_count: 0 });
    renderContainer();
    expect(await screen.findByTestId('dashboard-empty-state')).toHaveTextContent(
      'Sincroniza tus actividades de Strava para empezar',
    );
  });
});
