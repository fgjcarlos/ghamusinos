// useDashboardSummary: weekly KPIs + 7d/30d trends (DA-001).
// Fases 1.4 (issue #16, DW-002). The hook owns the request and the
// loading/error state; the container decides how to render it.

import { useEffect, useState } from 'react';
import { fetchDashboardSummary, type DashboardSummary } from '../../../lib/api/dashboard';
import { shareRequest } from './shareRequest';
import { describeError, initialAsyncState, type AsyncState } from './types';

export function useDashboardSummary(token: string): AsyncState<DashboardSummary> {
  const [state, setState] = useState<AsyncState<DashboardSummary>>(initialAsyncState);

  useEffect(() => {
    if (!token) {
      setState({
        data: null,
        loading: false,
        error: 'No se encontró token de autenticación. Configura VITE_AUTH_TOKEN en .env.',
      });
      return;
    }
    let cancelled = false;
    setState((prev) => ({ ...prev, loading: true, error: null }));
    shareRequest('dashboard-summary', () => fetchDashboardSummary(token))
      .then((data) => {
        if (cancelled) return;
        setState({ data, loading: false, error: null });
      })
      .catch((error: unknown) => {
        if (cancelled) return;
        setState({ data: null, loading: false, error: describeError(error) });
      });
    return () => {
      cancelled = true;
    };
  }, [token]);

  return state;
}
