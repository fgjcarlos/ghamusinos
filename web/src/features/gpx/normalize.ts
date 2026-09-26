// Normalize the StoredTrackDetail response into plain TypeScript
// shapes the presentational components can consume. Issue 157.
//
// The backend returns pgtype wrappers (UUID, Numeric, Int2) for some
// fields, and nested objects (`track.track` for inner metadata,
// `track.analysis` for the metrics). The presentational layer
// doesn't want to know any of that. This module is the boundary:
// it owns the conversion.

import type {
  GpxAnalysis,
  GpxClimb,
  GpxRiskZone,
  GpxTrackSummary,
  PgTypeUUID,
  StoredTrackDetail,
} from '../../lib/api/types';

export type DifficultyLabel = GpxAnalysis['difficulty_label'];

export interface NormalizedAnalysis {
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
  difficulty_label: DifficultyLabel;
  runnability_pct: number;
}

export interface NormalizedInnerTrack {
  id: string;
  user_id: string;
  name: string;
  file_hash: string;
  file_size_bytes: number;
  /** Track points are omitted in the list view; here they stay unknown[] for now. */
  points: unknown[];
  track_type: string;
  uploaded_at: Date;
}

export interface NormalizedClimb {
  id: string;
  start_idx: number;
  end_idx: number;
  gain_m: number;
  distance_m: number;
  avg_slope_pct: number;
  is_king_climb: boolean;
  vam: number | null;
}

export interface NormalizedRiskZone {
  start_idx: number;
  end_idx: number;
  category: GpxRiskZone['category'];
  severity: number;
}

export interface NormalizedTrackDetail {
  track: NormalizedInnerTrack;
  analysis: NormalizedAnalysis;
  climbs: NormalizedClimb[];
  risk_zones: NormalizedRiskZone[];
}

function pgUuidToString(u: PgTypeUUID | null | undefined): string {
  if (!u || !u.valid) return '';
  return u.bytes;
}

function normalizeAnalysis(a: GpxAnalysis): NormalizedAnalysis {
  return {
    distance_m: a.distance_m,
    moving_time_s: a.moving_time_s,
    d_plus_m: a.d_plus_m,
    d_minus_m: a.d_minus_m,
    // max/min_elevation_m are already number | null on GpxAnalysis
    // (only the climb fields and the inner-track id/UUIDs are pgtype).
    max_elevation_m: a.max_elevation_m,
    min_elevation_m: a.min_elevation_m,
    avg_slope_pct: a.avg_slope_pct,
    max_slope_pct: a.max_slope_pct,
    effort_index: a.effort_index,
    itra_points: a.itra_points,
    leg_breaker_index: a.leg_breaker_index,
    estimated_vam: a.estimated_vam,
    difficulty_score: a.difficulty_score,
    difficulty_label: a.difficulty_label,
    runnability_pct: a.runnability_pct,
  };
}

function normalizeInnerTrack(
  inner: GpxTrackSummary['track'],
): NormalizedInnerTrack {
  // uploaded_at is already an ISO 8601 string from the backend.
  const uploaded = inner.uploaded_at ? new Date(inner.uploaded_at) : new Date(0);
  return {
    id: pgUuidToString(inner.id),
    user_id: pgUuidToString(inner.user_id),
    name: inner.name,
    file_hash: inner.file_hash,
    file_size_bytes: inner.file_size_bytes,
    points: inner.points,
    track_type: inner.track_type,
    uploaded_at: uploaded,
  };
}

function normalizeClimb(c: GpxClimb): NormalizedClimb {
  return {
    id: pgUuidToString(c.id),
    start_idx: c.start_idx,
    end_idx: c.end_idx,
    // gain_m / distance_m / vam are already plain number on GpxClimb;
    // the backend has already done the pgtype conversion for these
    // fields, so no further wrapping is needed here.
    gain_m: c.gain_m,
    distance_m: c.distance_m,
    avg_slope_pct: c.avg_slope_pct,
    is_king_climb: c.is_king_climb,
    vam: c.vam,
  };
}

function normalizeRiskZone(r: GpxRiskZone): NormalizedRiskZone {
  return {
    start_idx: r.start_idx,
    end_idx: r.end_idx,
    category: r.category,
    severity: r.severity,
  };
}

/** Convert a StoredTrackDetail into the plain-shape NormalizedTrackDetail.
 *
 * Backend nests the inner track metadata under `detail.track.track`
 * and the analysis under `detail.track.analysis`. After normalization
 * the consumer sees a flat `track` + `analysis` pair. */
export function normalizeTrackDetail(
  detail: StoredTrackDetail,
): NormalizedTrackDetail {
  return {
    track: normalizeInnerTrack(detail.track.track),
    analysis: normalizeAnalysis(detail.track.analysis),
    climbs: detail.climbs.map(normalizeClimb),
    risk_zones: detail.risk_zones.map(normalizeRiskZone),
  };
}
