// TDD contract for CardiacDriftPanelContainer (Fase 1.4, DW-006).
// The panel is only mounted when the backend reports available=true;
// otherwise the parent shows the "needs HR streams" notice.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import type { CardiacDriftResponse } from '../../lib/api/dashboard';
import { ApiError } from '../../lib/api/types';
import { createDeferred } from '../../test/deferred';
import { CardiacDriftPanelContainer } from './CardiacDriftPanelContainer';

vi.mock('../../lib/api/dashboard', () => ({
  fetchDashboardSummary: vi.fn(),
  fetchTrainingLoad: vi.fn(),
  fetchHRZones: vi.fn(),
  fetchCardiacDrift: vi.fn(),
  postRecalc: vi.fn(),
}));

import * as dashboardApi from '../../lib/api/dashboard';

const mockedDrift = vi.mocked(dashboardApi.fetchCardiacDrift);

const AVAILABLE: CardiacDriftResponse = {
  from: '2026-01-01',
  to: '2026-01-31',
  available: true,
  activities: [{ activity_id: 'act-1', drift_pct: 4.2, sample_count: 900 }],
};

function renderContainer() {
  return render(<CardiacDriftPanelContainer from="2026-01-01" to="2026-01-31" />);
}

describe('CardiacDriftPanelContainer', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.stubEnv('VITE_AUTH_TOKEN', 'test-token');
  });

  it('renders the loading state while the drift is in flight', async () => {
    const deferred = createDeferred<CardiacDriftResponse>();
    mockedDrift.mockReturnValue(deferred.promise);
    const { unmount } = renderContainer();
    expect(screen.getByTestId('cardiac-drift-loading')).toBeInTheDocument();
    expect(screen.queryByTestId('cardiac-drift-panel')).not.toBeInTheDocument();
    unmount();
    deferred.resolve(AVAILABLE);
    await deferred.promise;
  });

  it('renders the error state when the drift fails', async () => {
    mockedDrift.mockRejectedValue(new ApiError(500, 'boom'));
    renderContainer();
    const error = await screen.findByTestId('cardiac-drift-error');
    expect(error).toHaveTextContent('Error 500: boom');
    expect(screen.queryByTestId('cardiac-drift-panel')).not.toBeInTheDocument();
  });

  it('mounts the panel when the drift is available', async () => {
    mockedDrift.mockResolvedValue(AVAILABLE);
    renderContainer();
    expect(await screen.findByTestId('cardiac-drift-panel')).toBeInTheDocument();
    expect(screen.queryByTestId('cardiac-drift-empty')).not.toBeInTheDocument();
  });

  it('shows the empty notice when the drift is not available', async () => {
    mockedDrift.mockResolvedValue({
      from: '2026-01-01',
      to: '2026-01-31',
      available: false,
      activities: [],
    });
    renderContainer();
    const notice = await screen.findByTestId('cardiac-drift-empty');
    expect(notice).toHaveTextContent('necesita streams HR para calcular');
    expect(screen.queryByTestId('cardiac-drift-panel')).not.toBeInTheDocument();
  });
});
