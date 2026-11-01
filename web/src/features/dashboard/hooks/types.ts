// Shared types for the dashboard hooks (Fase 1.4, DW-002).
//
// Every dashboard resource follows the same three-state contract:
// loading → (data | error). Keeping it in one place lets the
// containers render the three states without repeating the shape.

import { ApiError } from '../../../lib/api/types';

/**
 * AsyncState is the result of a dashboard hook.
 */
export interface AsyncState<T> {
  /** Payload once loaded; null while loading and after an error. */
  data: T | null;
  loading: boolean;
  /** Human-readable error message; null when there is no error. */
  error: string | null;
}

/**
 * describeError maps an unknown rejection into a message the UI can
 * show. ApiError carries the HTTP status; anything else (network
 * failure, aborted request) becomes a generic message.
 */
export function describeError(error: unknown): string {
  if (error instanceof ApiError) {
    return `Error ${error.status}: ${error.message}`;
  }
  return 'Error inesperado al cargar los datos del dashboard.';
}

/**
 * initialAsyncState builds the loading state of a resource.
 */
export function initialAsyncState<T>(): AsyncState<T> {
  return { data: null, loading: true, error: null };
}
