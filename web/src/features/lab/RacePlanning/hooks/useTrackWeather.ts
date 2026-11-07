import { useCallback, useRef, useState } from 'react';
import { fetchTrackWeather, type TrackWeatherResponse } from '../../../../lib/api/lab';
import { ApiError } from '../../../../lib/api/types';

export interface TrackWeatherState {
  data: TrackWeatherResponse | null;
  loading: boolean;
  error: string | null;
}

export interface TrackWeatherHook extends TrackWeatherState {
  load: (at: string) => Promise<void>;
  reset: () => void;
}

function messageFor(err: unknown): string {
  return err instanceof ApiError ? err.message : 'No se pudo cargar la previsión meteorológica';
}

/**
 * On-demand weather loader for one stored track. Nothing is fetched until
 * `load(at)` is called, and the last request wins when the user changes the
 * race date quickly.
 */
export function useTrackWeather(token: string, trackId: string): TrackWeatherHook {
  const [state, setState] = useState<TrackWeatherState>({
    data: null,
    loading: false,
    error: null,
  });
  const requestId = useRef(0);

  const load = useCallback(
    async (at: string) => {
      if (!token || !trackId) {
        setState({ data: null, loading: false, error: 'Falta el token o el track' });
        return;
      }
      const current = requestId.current + 1;
      requestId.current = current;
      setState({ data: null, loading: true, error: null });
      try {
        const data = await fetchTrackWeather(token, trackId, at);
        if (requestId.current === current) {
          setState({ data, loading: false, error: null });
        }
      } catch (err: unknown) {
        if (requestId.current === current) {
          setState({ data: null, loading: false, error: messageFor(err) });
        }
      }
    },
    [token, trackId],
  );

  const reset = useCallback(() => {
    requestId.current += 1;
    setState({ data: null, loading: false, error: null });
  }, []);

  return { ...state, load, reset };
}
