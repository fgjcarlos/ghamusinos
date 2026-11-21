import { ApiError } from './types';
import type { PaginatedWorkouts, Workout, WorkoutInput, WorkoutPatch } from './workout-types';

const BASE_URL = '/api/v1/workouts';
const auth = (token: string): HeadersInit => ({
  Authorization: `Bearer ${token}`,
  'Content-Type': 'application/json',
});

async function json<T>(response: Response): Promise<T> {
  if (!response.ok) {
    let problem: {
      detail?: string;
      title?: string;
      instance?: string;
      errors?: Record<string, string>;
    } = {};
    try {
      const body: unknown = await response.json();
      if (body && typeof body === 'object' && !Array.isArray(body)) {
        problem = body as typeof problem;
      }
    } catch {
      /* use status fallback */
    }
    throw new ApiError(
      response.status,
      problem.detail ?? problem.title ?? `HTTP ${response.status}`,
      problem.errors,
      problem.instance,
    );
  }
  return response.json() as Promise<T>;
}

export async function listWorkouts(
  token: string,
  from: string,
  to: string,
  limit = 20,
  offset = 0,
  signal?: AbortSignal,
): Promise<PaginatedWorkouts> {
  const query = new URLSearchParams({ from, to, limit: String(limit), offset: String(offset) });
  const init: RequestInit = { method: 'GET', headers: auth(token) };
  if (signal) init.signal = signal;
  return json<PaginatedWorkouts>(await fetch(`${BASE_URL}?${query}`, init));
}

export async function createWorkout(token: string, workout: WorkoutInput): Promise<Workout> {
  return json<Workout>(
    await fetch(BASE_URL, { method: 'POST', headers: auth(token), body: JSON.stringify(workout) }),
  );
}

export async function updateWorkout(
  token: string,
  id: string,
  workout: WorkoutPatch,
): Promise<Workout> {
  return json<Workout>(
    await fetch(`${BASE_URL}/${encodeURIComponent(id)}`, {
      method: 'PATCH',
      headers: auth(token),
      body: JSON.stringify(workout),
    }),
  );
}

export async function deleteWorkout(token: string, id: string): Promise<void> {
  const response = await fetch(`${BASE_URL}/${encodeURIComponent(id)}`, {
    method: 'DELETE',
    headers: auth(token),
  });
  if (!response.ok) await json<unknown>(response);
}

export async function linkWorkoutActivity(
  token: string,
  id: string,
  activityId: string,
): Promise<Workout> {
  return json<Workout>(
    await fetch(`${BASE_URL}/${encodeURIComponent(id)}/activity`, {
      method: 'PUT',
      headers: auth(token),
      body: JSON.stringify({ activity_id: activityId }),
    }),
  );
}

export async function unlinkWorkoutActivity(token: string, id: string): Promise<Workout> {
  return json<Workout>(
    await fetch(`${BASE_URL}/${encodeURIComponent(id)}/activity`, {
      method: 'DELETE',
      headers: auth(token),
    }),
  );
}
