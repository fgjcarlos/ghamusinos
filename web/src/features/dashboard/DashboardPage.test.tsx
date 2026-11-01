// TDD contract for DashboardPage (Fase 1.4, DW-001..006).
// Composition order, the period selector and the global empty state.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import type {
  CardiacDriftResponse,
  DashboardSummary,
  HRZonesResponse,
  TrainingLoadSeries,
} from '../../lib/api/dashboard';
import { DashboardPage } from './DashboardPage';

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
const mockedZones = vi.mocked(dashboardApi.fetchHRZones);
const mockedDrift = vi.mocked(dashboardApi.fetchCardiacDrift);

const SUMMARY: DashboardSummary = {
  weekly_volume_m: 1000,
  weekly_elevation_m: 100,
  weekly_activities_count: 3,
  trend_7d_pct: 1,
  trend_30d_pct: 2,
};

const LOAD: TrainingLoadSeries = {
  from: '2026-01-01',
  to: '2026-01-31',
  series: [{ day: '2026-01-31', ctl: 42, atl: 47, tsb: -5, tss: 30 }],
};

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

const DRIFT: CardiacDriftResponse = {
  from: '2026-01-01',
  to: '2026-01-31',
  available: true,
  activities: [{ activity_id: 'act-1', drift_pct: 4.2, sample_count: 900 }],
};

/** Number of days covered by an inclusive [from, to] window. */
function daysBetween(from: string, to: string): number {
  const MS_PER_DAY = 24 * 60 * 60 * 1000;
  return (
    Math.round((Date.parse(`${to}T00:00:00Z`) - Date.parse(`${from}T00:00:00Z`)) / MS_PER_DAY) + 1
  );
}

describe('DashboardPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.stubEnv('VITE_AUTH_TOKEN', 'test-token');
    mockedSummary.mockResolvedValue(SUMMARY);
    mockedLoad.mockResolvedValue(LOAD);
    mockedZones.mockResolvedValue(ZONES);
    mockedDrift.mockResolvedValue(DRIFT);
  });

  it('renders the period selector with 30d selected by default', () => {
    render(<DashboardPage />);
    expect(screen.getByTestId('period-selector')).toBeInTheDocument();
    expect(screen.getByTestId('period-option-30d')).toHaveAttribute('aria-pressed', 'true');
  });

  it('composes the summary card, the load chart, the HR zones chart and the drift panel in order', async () => {
    render(<DashboardPage />);
    await screen.findByTestId('cardiac-drift-panel');

    const order = [
      'dashboard-summary-card',
      'training-load-chart',
      'hr-zones-chart',
      'cardiac-drift-panel',
    ];
    const nodes = order.map((testId) => screen.getByTestId(testId));
    for (let i = 0; i < nodes.length - 1; i += 1) {
      const position = nodes[i].compareDocumentPosition(nodes[i + 1]);
      expect(position & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    }
  });

  it('re-queries the charts with the new range when the period changes', async () => {
    render(<DashboardPage />);
    await screen.findByTestId('training-load-chart');

    const initialCall = mockedLoad.mock.calls[0];
    expect(initialCall).toBeDefined();
    expect(daysBetween(initialCall![1], initialCall![2])).toBe(30);

    fireEvent.click(screen.getByTestId('period-option-7d'));

    await waitFor(() => {
      const lastCall = mockedLoad.mock.calls[mockedLoad.mock.calls.length - 1];
      expect(lastCall).toBeDefined();
      expect(daysBetween(lastCall![1], lastCall![2])).toBe(7);
    });
    expect(mockedLoad.mock.calls[mockedLoad.mock.calls.length - 1]![0]).toBe('test-token');
  });

  it('renders the global empty state and no charts when there are no activities', async () => {
    mockedSummary.mockResolvedValue({ ...SUMMARY, weekly_activities_count: 0 });
    render(<DashboardPage />);

    const empty = await screen.findByTestId('dashboard-empty-state');
    expect(empty).toHaveTextContent('Sincroniza tus actividades de Strava para empezar');
    expect(screen.queryByTestId('training-load-chart')).not.toBeInTheDocument();
    expect(screen.queryByTestId('hr-zones-chart')).not.toBeInTheDocument();
    expect(screen.queryByTestId('cardiac-drift-panel')).not.toBeInTheDocument();
  });

  it('shows the drift notice instead of the panel when the drift is not available', async () => {
    mockedDrift.mockResolvedValue({
      from: '2026-01-01',
      to: '2026-01-31',
      available: false,
      activities: [],
    });
    render(<DashboardPage />);
    const notice = await screen.findByTestId('cardiac-drift-empty');
    expect(notice).toHaveTextContent('necesita streams HR para calcular');
    expect(screen.queryByTestId('cardiac-drift-panel')).not.toBeInTheDocument();
  });
});
