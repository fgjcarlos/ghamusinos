// Test helper: a promise whose settlement is controlled by the test.
//
// The dashboard containers have a loading state; to exercise it the
// request must stay in flight while the assertions run, and settle
// afterwards so the shared in-flight request map is cleaned up.

export interface Deferred<T> {
  promise: Promise<T>;
  resolve: (value: T) => void;
  reject: (reason?: unknown) => void;
}

export function createDeferred<T>(): Deferred<T> {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}
