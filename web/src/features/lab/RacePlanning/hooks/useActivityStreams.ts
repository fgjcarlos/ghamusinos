import { useCallback, useRef, useState } from 'react';
import {
  fetchActivityStreams,
  refreshActivityStreams,
  type ActivityStreamsResponse,
} from '../../../../lib/api/lab';
import { ApiError } from '../../../../lib/api/types';

export type ActivityStreamsState =
  | { kind: 'idle' }
  | { kind: 'loading' }
  | { kind: 'ready'; data: ActivityStreamsResponse }
  | { kind: 'needs-refresh'; detail: string }
  | { kind: 'error'; message: string };

export interface ActivityStreamsHook {
  state: ActivityStreamsState;
  load: () => Promise<void>;
  refresh: () => Promise<void>;
}

/**
 * Loads the per-point streams of one Strava activity for Post-Activity.
 * A 409 means the activity predates the `time` import; `refresh()` enqueues
 * the re-import and leaves the section in `needs-refresh` until the user
 * retries.
 */
export function useActivityStreams(token: string, activityExternalId: string): ActivityStreamsHook {
  const [state, setState] = useState<ActivityStreamsState>({ kind: 'idle' });
  const requestId = useRef(0);

  const load = useCallback(async () => {
    if (!token || !activityExternalId) {
      setState({ kind: 'error', message: 'Falta el token o la actividad' });
      return;
    }
    const current = requestId.current + 1;
    requestId.current = current;
    setState({ kind: 'loading' });
    try {
      const data = await fetchActivityStreams(token, activityExternalId);
      if (requestId.current === current) {
        setState({ kind: 'ready', data });
      }
    } catch (err: unknown) {
      if (requestId.current !== current) return;
      if (err instanceof ApiError && err.status === 409) {
        setState({ kind: 'needs-refresh', detail: err.message });
        return;
      }
      setState({
        kind: 'error',
        message:
          err instanceof ApiError
            ? err.message
            : 'No se pudieron cargar los streams de la actividad',
      });
    }
  }, [token, activityExternalId]);

  const refresh = useCallback(async () => {
    if (!token || !activityExternalId) return;
    requestId.current += 1;
    setState({ kind: 'loading' });
    try {
      await refreshActivityStreams(token, activityExternalId);
      setState({
        kind: 'needs-refresh',
        detail: 'Re-sincronización en curso. Vuelve a intentarlo en unos segundos.',
      });
    } catch (err: unknown) {
      setState({
        kind: 'error',
        message:
          err instanceof ApiError
            ? err.message
            : 'No se pudo encolar la re-sincronización de streams',
      });
    }
  }, [token, activityExternalId]);

  return { state, load, refresh };
}
