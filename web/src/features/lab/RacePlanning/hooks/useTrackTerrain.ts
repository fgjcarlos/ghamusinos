import { useCallback, useRef, useState } from 'react';
import { fetchTrackTerrain, type TrackTerrainResponse } from '../../../../lib/api/lab';
import { ApiError } from '../../../../lib/api/types';

export interface TrackTerrainState {
  data: TrackTerrainResponse | null;
  loading: boolean;
  error: string | null;
}

export interface TrackTerrainHook extends TrackTerrainState {
  load: () => Promise<void>;
  reset: () => void;
}

function messageFor(err: unknown): string {
  return err instanceof ApiError ? err.message : 'No se pudo analizar el terreno';
}

/**
 * On-demand terrain loader for one stored track. The backend caches the
 * Overpass response for 24 h; the hook only owns the request lifecycle.
 */
export function useTrackTerrain(token: string, trackId: string): TrackTerrainHook {
  const [state, setState] = useState<TrackTerrainState>({
    data: null,
    loading: false,
    error: null,
  });
  const requestId = useRef(0);

  const load = useCallback(async () => {
    if (!token || !trackId) {
      setState({ data: null, loading: false, error: 'Falta el token o el track' });
      return;
    }
    const current = requestId.current + 1;
    requestId.current = current;
    setState({ data: null, loading: true, error: null });
    try {
      const data = await fetchTrackTerrain(token, trackId);
      if (requestId.current === current) {
        setState({ data, loading: false, error: null });
      }
    } catch (err: unknown) {
      if (requestId.current === current) {
        setState({ data: null, loading: false, error: messageFor(err) });
      }
    }
  }, [token, trackId]);

  const reset = useCallback(() => {
    requestId.current += 1;
    setState({ data: null, loading: false, error: null });
  }, []);

  return { ...state, load, reset };
}
