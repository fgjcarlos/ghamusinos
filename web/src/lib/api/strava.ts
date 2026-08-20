/**
 * Native fetch-based API client for Strava endpoints.
 * All functions accept a bearer token (JWT) as a parameter.
 *
 * Note: No Clerk React SDK is installed yet; authentication is handled
 * by passing tokens explicitly. This should be integrated with Clerk
 * once the authentication layer is set up.
 * TODO(#90): Integrate Clerk React SDK for production auth.
 */

import { Activity, PaginatedActivities, SyncSession, ApiError } from './types';

const BASE_URL = '/api/v1';

/**
 * makeAuthHeader creates an Authorization header with Bearer token.
 */
function makeAuthHeader(token: string): HeadersInit {
  return {
    Authorization: `Bearer ${token}`,
    'Content-Type': 'application/json',
  };
}

/**
 * handleResponse converts a Response to JSON or throws an ApiError.
 */
async function handleResponse<T>(response: Response): Promise<T> {
  if (!response.ok) {
    const errorMessage = `HTTP ${response.status}`;
    throw new ApiError(response.status, errorMessage);
  }
  return response.json() as Promise<T>;
}

/**
 * listActivities fetches a paginated list of activities for the authenticated user.
 * @param token Bearer token for authorization
 * @param page Page number (1-indexed); defaults to 1
 * @param limit Number of activities per page; defaults to 20
 */
export async function listActivities(
  token: string,
  page: number = 1,
  limit: number = 20,
): Promise<PaginatedActivities> {
  const params = new URLSearchParams({
    page: String(page),
    limit: String(limit),
  });
  const response = await fetch(`${BASE_URL}/activities?${params}`, {
    method: 'GET',
    headers: makeAuthHeader(token),
  });
  return handleResponse<PaginatedActivities>(response);
}

/**
 * getActivity fetches a single activity by ID.
 * @param token Bearer token for authorization
 * @param id Activity ID (UUID as string, or the full pgtype.UUID object converted to string)
 */
export async function getActivity(token: string, id: string): Promise<Activity> {
  const response = await fetch(`${BASE_URL}/activities/${id}`, {
    method: 'GET',
    headers: makeAuthHeader(token),
  });
  return handleResponse<Activity>(response);
}

/**
 * getSyncStatus fetches the current sync session status for the authenticated user.
 */
export async function getSyncStatus(token: string): Promise<SyncSession> {
  const response = await fetch(`${BASE_URL}/sync/status`, {
    method: 'GET',
    headers: makeAuthHeader(token),
  });
  return handleResponse<SyncSession>(response);
}
