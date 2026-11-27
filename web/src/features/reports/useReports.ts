// Data hook of the reports feature. Same three-state contract as
// features/races/useRaceData, with a stable refresh: the pending
// generation effect depends on it to poll a bounded number of times.

import { useCallback, useEffect, useState } from 'react';

export type ReportsResult<T> =
  { phase: 'loading' } | { phase: 'error' } | { phase: 'ready'; data: T };

export function useReportsData<T>(load: (signal: AbortSignal) => Promise<T>) {
  const [result, setResult] = useState<ReportsResult<T>>({ phase: 'loading' });
  const [attempt, setAttempt] = useState(0);
  const refresh = useCallback(() => setAttempt((value) => value + 1), []);
  useEffect(() => {
    const controller = new AbortController();
    setResult({ phase: 'loading' });
    void load(controller.signal).then(
      (data) => {
        if (!controller.signal.aborted) setResult({ phase: 'ready', data });
      },
      () => {
        if (!controller.signal.aborted) setResult({ phase: 'error' });
      },
    );
    return () => controller.abort();
  }, [load, attempt]);
  return { result, refresh };
}
