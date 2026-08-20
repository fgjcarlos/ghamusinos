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

/**
 * getStravaAuthorizeURL asks the backend for the URL the browser should
 * navigate to in order to start the Strava OAuth flow.
 *
 * The backend returns {"authorize_url": "https://www.strava.com/oauth/authorize?..."}.
 * The user_id is embedded inside the signed state (HMAC-SHA256, AUD-02), so
 * the browser-top-level redirect that follows does not need to carry any
 * session cookie or Authorization header.
 *
 * The caller is expected to do window.location.assign(url) — NOT to set the
 * URL as <a href>, because a navigation top-level does not carry headers and
 * the old /api/v1/strava/connect (which used to 302) was unreachable from a
 * plain anchor for that reason.
 */
export async function getStravaAuthorizeURL(token: string): Promise<string> {
  const response = await fetch(`${BASE_URL}/strava/connect`, {
    method: 'GET',
    headers: makeAuthHeader(token),
  });
  const body = await handleResponse<{ authorize_url: string }>(response);
  return body.authorize_url;
}
