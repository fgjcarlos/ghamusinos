// TDD contract for TrainingLoadChartContainer (Fase 1.4, DW-004).
// The container owns the data: loading, error and populated states are
// visually distinct from the populated chart.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import type { TrainingLoadSeries } from '../../lib/api/dashboard';
import { ApiError } from '../../lib/api/types';
import { createDeferred } from '../../test/deferred';
import { TrainingLoadChartContainer } from './TrainingLoadChartContainer';

vi.mock('../../lib/api/dashboard', () => ({
  fetchDashboardSummary: vi.fn(),
  fetchTrainingLoad: vi.fn(),
  fetchHRZones: vi.fn(),
  fetchCardiacDrift: vi.fn(),
  postRecalc: vi.fn(),
}));

import * as dashboardApi from '../../lib/api/dashboard';

const mockedLoad = vi.mocked(dashboardApi.fetchTrainingLoad);

const SERIES: TrainingLoadSeries = {
  from: '2026-01-01',
  to: '2026-01-31',
  series: [{ day: '2026-01-31', ctl: 42, atl: 47, tsb: -5, tss: 30 }],
};

function renderContainer() {
  return render(<TrainingLoadChartContainer from="2026-01-01" to="2026-01-31" />);
}

describe('TrainingLoadChartContainer', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.stubEnv('VITE_AUTH_TOKEN', 'test-token');
  });

  it('renders the loading state while the series is in flight', async () => {
    const deferred = createDeferred<TrainingLoadSeries>();
    mockedLoad.mockReturnValue(deferred.promise);
    const { unmount } = renderContainer();
    expect(screen.getByTestId('training-load-loading')).toBeInTheDocument();
    expect(screen.queryByTestId('training-load-chart')).not.toBeInTheDocument();
    unmount();
    deferred.resolve(SERIES);
    await deferred.promise;
  });

  it('renders the error state when the series fails', async () => {
    mockedLoad.mockRejectedValue(new ApiError(503, 'no data'));
    renderContainer();
    const error = await screen.findByTestId('training-load-error');
    expect(error).toHaveTextContent('Error 503: no data');
    expect(screen.queryByTestId('training-load-chart')).not.toBeInTheDocument();
  });

  it('renders the chart when the series arrive', async () => {
    mockedLoad.mockResolvedValue(SERIES);
    renderContainer();
    expect(await screen.findByTestId('training-load-chart')).toBeInTheDocument();
    expect(screen.getByTestId('training-load-legend')).toHaveTextContent('CTL');
  });
});
