// useCardiacDrift: per-activity cardiac drift for a window (DA-007).
// Fases 1.4 (issue #16, DW-002). The hook owns the request and the
// loading/error state; the container decides how to render it.

import { useEffect, useState } from 'react';
import { fetchCardiacDrift, type CardiacDriftResponse } from '../../../lib/api/dashboard';
import { shareRequest } from './shareRequest';
import { describeError, initialAsyncState, type AsyncState } from './types';

export function useCardiacDrift(
  token: string,
  from: string,
  to: string,
): AsyncState<CardiacDriftResponse> {
  const [state, setState] = useState<AsyncState<CardiacDriftResponse>>(initialAsyncState);

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
    shareRequest(`cardiac-drift:${from}:${to}`, () => fetchCardiacDrift(token, from, to))
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
