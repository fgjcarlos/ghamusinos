import { useEffect, useState } from 'react';

type Result<T> = { phase: 'loading' } | { phase: 'error' } | { phase: 'ready'; data: T };

export function useRaceData<T>(load: (signal: AbortSignal) => Promise<T>) {
  const [result, setResult] = useState<Result<T>>({ phase: 'loading' });
  const [attempt, setAttempt] = useState(0);
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
  return { result, retry: () => setAttempt((value) => value + 1) };
}
