// useTrainingLoad: daily CTL/ATL/TSB series for a window (DA-002).
// Fases 1.4 (issue #16, DW-002). The hook owns the request and the
// loading/error state; the container decides how to render it.

import { useEffect, useState } from 'react';
import { fetchTrainingLoad, type TrainingLoadSeries } from '../../../lib/api/dashboard';
import { shareRequest } from './shareRequest';
import { describeError, initialAsyncState, type AsyncState } from './types';

export function useTrainingLoad(
  token: string,
  from: string,
  to: string,
): AsyncState<TrainingLoadSeries> {
  const [state, setState] = useState<AsyncState<TrainingLoadSeries>>(initialAsyncState);

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
    shareRequest(`training-load:${from}:${to}`, () => fetchTrainingLoad(token, from, to))
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
  }, [token, from, to]);

  return state;
}
