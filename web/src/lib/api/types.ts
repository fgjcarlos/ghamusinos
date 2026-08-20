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
  status: 'pending' | 'running' | 'completed' | 'failed' | 'cancelled';
  window_days: number;
  total_activities: number;
  imported: number;
  skipped: number;
  error: string | null;
  started_at: string; // RFC3339 timestamp
  finished_at: string | null; // RFC3339 timestamp, null if still running
}

/**
 * ApiError is thrown when an API call fails.
 * It carries the HTTP status code and a descriptive message.
 */
export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
    this.name = 'ApiError';
    Object.setPrototypeOf(this, ApiError.prototype);
  }
}

// ─────────────────────────────────────────────────────────────────
// GPX / Laboratorio
// ─────────────────────────────────────────────────────────────────

/**
 * Análisis numérico de un track GPX. Mismas reglas que el resto del
 * repo: pgtype wrappers se respetan para no introducir el bug que ya
 * tenían los endpoints de Strava (TODO #90 sobre flatten en backend).
 */
export interface GpxAnalysis {
  distance_m: number;
  moving_time_s: number;
  d_plus_m: number;
  d_minus_m: number;
  max_elevation_m: number | null;
  min_elevation_m: number | null;
  avg_slope_pct: number;
  max_slope_pct: number;
  effort_index: number;
  itra_points: number;
  leg_breaker_index: number;
  estimated_vam: number;
  difficulty_score: number;
  difficulty_label: 'easy' | 'moderate' | 'hard' | 'extreme';
  runnability_pct: number;
}

export interface GpxClimb {
  id: PgTypeUUID;
  start_idx: number;
  end_idx: number;
  gain_m: number;
  distance_m: number;
  avg_slope_pct: number;
  is_king_climb: boolean;
  vam: number | null;
}

export interface GpxRiskZone {
  // El shape exacto depende de internal/gpx/types.go:RiskZone; este
  // stub evita acoplar el cliente al backend hasta que #125 lo pida.
  start_idx: number;
  end_idx: number;
  category: 'steep' | 'technical' | 'exposure';
  severity: number;
}

/**
 * Track resumido: lo que devuelve ListGPX (cada item de data).
 * No incluye climbs ni risk_zones — están en StoredTrackDetail.
 */
export interface GpxTrackSummary {
  track: {
    id: PgTypeUUID;
    user_id: PgTypeUUID;
    name: string;
    file_hash: string;
    file_size_bytes: number;
    points: unknown[]; // omitido por la API en List; array vacío para no chocar tipos
    track_type: string;
    uploaded_at: string;
  };
  analysis: GpxAnalysis;
}

/**
 * Track con detalle: lo que devuelve GetGPX/{id}.
 */
export interface StoredTrackDetail {
  track: GpxTrackSummary;
  climbs: GpxClimb[];
  risk_zones: GpxRiskZone[];
}

export interface PaginatedTracks {
  data: GpxTrackSummary[];
  total: number;
  limit: number;
  offset: number;
  has_next: boolean;
}

/**
 * Diff de compareGPX: una métrica por key (distance_m, d_plus_m,
 * etc.) con sus valores por track (mismo orden que el array de IDs)
 * y el índice del "mejor" track. El shape está definido en
 * internal/http/handlers/gpx_compare.go (computeDiff).
 */
export interface CompareMetric {
  values: number[];
  best_track: number;
  unit: string;
}

export interface CompareResponse {
  tracks: StoredTrackDetail[];
  diff: Record<string, CompareMetric>;
}
