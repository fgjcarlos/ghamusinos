/**
 * Type definitions for Strava API responses.
 *
 * Note: pgtype serialization produces wrapper objects for UUID, Numeric, and Int2.
 * This is a known limitation that should be fixed in the backend DTO layer.
 * TODO(#90): flatten API response in backend to return clean scalar types.
 */

/**
 * pgtype.UUID serializes to {bytes: string, valid: boolean}.
 * This is the actual JSON shape returned by the API.
 */
export interface PgTypeUUID {
  bytes: string;
  valid: boolean;
}

/**
 * pgtype.Numeric serializes to {bigint: string, exp: number, sign: number, valid: boolean}.
 * Used for decimal/high-precision numbers in the database.
 */
export interface PgTypeNumeric {
  bigint: string;
  exp: number;
  sign: number;
  valid: boolean;
}

/**
 * pgtype.Int2 serializes to {int16: number, valid: boolean}.
 * Used for small integers (e.g., HR values).
 */
export interface PgTypeInt2 {
  int16: number;
  valid: boolean;
}

/**
 * Activity represents a single Strava activity with all its metadata.
 * IDs and timestamps use the wrapped pgtype serialization format.
 */
export interface Activity {
  id: PgTypeUUID;
  user_id: PgTypeUUID;
  external_source: string; // e.g., "strava"
  external_id: number;
  name: string;
  sport_type: string; // e.g., "Run", "Ride", "Swim"
  started_at: string; // RFC3339 timestamp
  elapsed_seconds: number;
  moving_seconds: number;
  distance_meters: PgTypeNumeric | null;
  elevation_gain_m: PgTypeNumeric | null;
  avg_hr: PgTypeInt2 | null;
  max_hr: PgTypeInt2 | null;
  avg_power: PgTypeInt2 | null;
  raw_payload: unknown; // Raw Strava JSON stored for reference
  created_at: string; // RFC3339 timestamp
  updated_at: string; // RFC3339 timestamp
}

/**
 * PaginatedActivities is the response format for the activities list endpoint.
 */
export interface PaginatedActivities {
  data: Activity[];
  page: number;
  total: number;
  has_next: boolean;
}

/**
 * SyncSession tracks the progress of a Strava activity import/sync job.
 */
export interface SyncSession {
  id: string;
  user_id: string;
  status: "pending" | "running" | "completed" | "failed" | "cancelled";
  window_days: number;
  total_activities: number;
  imported: number;
  skipped: number;
  error: string | null;
  started_at: string; // RFC3339 timestamp
  finished_at: string | null; // RFC3339 timestamp, null if still running
}

/**
 * ConnectResponse is returned by GET /api/v1/strava/connect.
 * It contains the OAuth redirect URL and a CSRF state token.
 */
export interface ConnectResponse {
  authorize_url: string;
  state: string;
}

/**
 * ApiError is thrown when an API call fails.
 * It carries the HTTP status code and a descriptive message.
 */
export class ApiError extends Error {
  constructor(
    public status: number,
    message: string
  ) {
    super(message);
    this.name = "ApiError";
    Object.setPrototypeOf(this, ApiError.prototype);
  }
}
