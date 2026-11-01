// In-flight request sharing for the dashboard hooks.
//
// Two components of the dashboard need the same resource at the same
// time: the page needs the summary to decide whether there is data at
// all, and the summary card needs it to draw the tiles; the summary
// card and the load chart both need the training-load series (the
// sparkline reads its last 7 days). Without sharing, the page would
// fire the same request twice for every period change.
//
// The promise is dropped from the map as soon as it settles, so a new
// mount always asks the network again (no stale cache).

const inFlight = new Map<string, Promise<unknown>>();

/**
 * shareRequest runs `request` unless an identical request (same key)
 * is already pending, in which case the pending promise is reused.
 */
export function shareRequest<T>(key: string, request: () => Promise<T>): Promise<T> {
  const pending = inFlight.get(key);
  if (pending) {
    return pending as Promise<T>;
  }
  const promise = request().finally(() => {
    inFlight.delete(key);
  });
  inFlight.set(key, promise);
  return promise;
}
