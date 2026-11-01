// TDD contract for HRZonesChartContainer (Fase 1.4, DW-004).
// The container owns the data and forwards the `degraded` flag, which
// decides whether the approximation label is shown.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import type { HRZonesResponse } from '../../lib/api/dashboard';
import { ApiError } from '../../lib/api/types';
import { createDeferred } from '../../test/deferred';
import { HRZonesChartContainer } from './HRZonesChartContainer';

vi.mock('../../lib/api/dashboard', () => ({
  fetchDashboardSummary: vi.fn(),
  fetchTrainingLoad: vi.fn(),
  fetchHRZones: vi.fn(),
  fetchCardiacDrift: vi.fn(),
  postRecalc: vi.fn(),
}));

import * as dashboardApi from '../../lib/api/dashboard';

const mockedZones = vi.mocked(dashboardApi.fetchHRZones);

const ZONES: HRZonesResponse = {
  from: '2026-01-01',
  to: '2026-01-31',
  degraded: false,
  zones: [
    { zone: 'z1', minutes: 10 },
    { zone: 'z2', minutes: 20 },
    { zone: 'z3', minutes: 30 },
    { zone: 'z4', minutes: 40 },
    { zone: 'z5', minutes: 50 },
  ],
};

function renderContainer() {
  return render(<HRZonesChartContainer from="2026-01-01" to="2026-01-31" />);
}

describe('HRZonesChartContainer', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.stubEnv('VITE_AUTH_TOKEN', 'test-token');
  });

  it('renders the loading state while the zones are in flight', async () => {
    const deferred = createDeferred<HRZonesResponse>();
    mockedZones.mockReturnValue(deferred.promise);
    const { unmount } = renderContainer();
    expect(screen.getByTestId('hr-zones-loading')).toBeInTheDocument();
    expect(screen.queryByTestId('hr-zones-chart')).not.toBeInTheDocument();
    unmount();
    deferred.resolve(ZONES);
    await deferred.promise;
  });

  it('renders the error state when the zones fail', async () => {
    mockedZones.mockRejectedValue(new ApiError(500, 'boom'));
    renderContainer();
    const error = await screen.findByTestId('hr-zones-error');
    expect(error).toHaveTextContent('Error 500: boom');
    expect(screen.queryByTestId('hr-zones-chart')).not.toBeInTheDocument();
  });

  it('renders the chart without the approximation label when the zones are exact', async () => {
    mockedZones.mockResolvedValue(ZONES);
    renderContainer();
    expect(await screen.findByTestId('hr-zones-chart')).toBeInTheDocument();
    expect(screen.queryByTestId('hr-zones-degraded-label')).not.toBeInTheDocument();
  });

  it('renders the approximation label when the zones are degraded', async () => {
    mockedZones.mockResolvedValue({ ...ZONES, degraded: true });
    renderContainer();
    const label = await screen.findByTestId('hr-zones-degraded-label');
    expect(label).toHaveTextContent('aproximación (sin streams HR)');
  });
});
