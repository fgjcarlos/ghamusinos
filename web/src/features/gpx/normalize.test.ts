// TDD contract for normalize (issue 157).
// Pure function tests: pgtype wrappers + nested StoredTrackDetail → plain TS.

import { describe, it, expect } from 'vitest';
import { normalizeTrackDetail } from './normalize';
import type {
  GpxAnalysis,
  GpxClimb,
  GpxRiskZone,
  GpxTrackSummary,
  StoredTrackDetail,
} from '../../lib/api/types';

const pgUuid = (bytes: string): { bytes: string; valid: boolean } => ({ bytes, valid: true });

const analysis: GpxAnalysis = {
  distance_m: 12345,
  moving_time_s: 3600,
  d_plus_m: 500,
  d_minus_m: 480,
  // max/min_elevation are plain number | null on GpxAnalysis
  // (the pgtype wrappers live on the climb fields and the inner track).
  max_elevation_m: 1500,
  min_elevation_m: 300,
  elevation_coverage: null,
  avg_slope_pct: 4.2,
  max_slope_pct: 18.7,
  effort_index: 87.3,
  itra_points: 320,
  leg_breaker_index: 12.4,
  estimated_vam: 850,
  difficulty_score: 75,
  difficulty_label: 'hard',
  runnability_pct: 78.5,
};

const trackSummary: GpxTrackSummary = {
  track: {
    id: pgUuid('0'.repeat(32)),
    user_id: pgUuid('1'.repeat(32)),
    name: 'Circular del Torrico',
    file_hash: 'abc123',
    file_size_bytes: 102400,
    points: [],
    track_type: 'circular',
    uploaded_at: '2025-09-15T08:00:00Z',
  },
  analysis,
};

const climbs: GpxClimb[] = [
  {
    id: pgUuid('a'.repeat(32)),
    start_idx: 10,
    end_idx: 20,
    // gain_m / distance_m / vam are plain number on GpxClimb.
    gain_m: 200,
    distance_m: 2000,
    avg_slope_pct: 10,
    is_king_climb: true,
    vam: 850,
  },
];

const risks: GpxRiskZone[] = [{ start_idx: 30, end_idx: 35, category: 'steep', severity: 0.9 }];

const detail: StoredTrackDetail = {
  track: trackSummary,
  climbs,
  risk_zones: risks,
  muros: [],
  recovery_zones: [],
  km_vertical: null,
};

describe('normalizeTrackDetail', () => {
  it('converts the inner-track id and user_id from pgtype to string', () => {
    const out = normalizeTrackDetail(detail);
    expect(out.track.id).toBe('0'.repeat(32));
    expect(out.track.user_id).toBe('1'.repeat(32));
  });

  it('passes through the analysis primitives unchanged', () => {
    const out = normalizeTrackDetail(detail);
    expect(out.analysis.distance_m).toBe(12345);
    expect(out.analysis.difficulty_label).toBe('hard');
    expect(out.analysis.difficulty_score).toBe(75);
  });

  it('passes through max/min elevation as plain number | null', () => {
    const signed: GpxAnalysis = {
      ...analysis,
      max_elevation_m: 1500,
      min_elevation_m: -50, // signed negative
    };
    const signedSummary: GpxTrackSummary = { ...trackSummary, analysis: signed };
    const out = normalizeTrackDetail({
      track: signedSummary,
      climbs,
      risk_zones: risks,
      muros: [],
      recovery_zones: [],
      km_vertical: null,
    });
    expect(out.analysis.max_elevation_m).toBe(1500);
    expect(out.analysis.min_elevation_m).toBe(-50);
  });

  it('preserves null for an absent elevation on analysis', () => {
    const noElev: GpxAnalysis = {
      ...analysis,
      max_elevation_m: null,
    };
    const noElevSummary: GpxTrackSummary = { ...trackSummary, analysis: noElev };
    const out = normalizeTrackDetail({
      track: noElevSummary,
      climbs,
      risk_zones: risks,
      muros: [],
      recovery_zones: [],
      km_vertical: null,
    });
    expect(out.analysis.max_elevation_m).toBeNull();
  });

  it('preserves elevation coverage values and nulls', () => {
    const withCoverage: GpxAnalysis = { ...analysis, elevation_coverage: 0.87 };
    const out = normalizeTrackDetail({
      track: { ...trackSummary, analysis: withCoverage },
      climbs,
      risk_zones: risks,
      muros: [],
      recovery_zones: [],
      km_vertical: null,
    });
    expect(out.analysis.elevation_coverage).toBe(0.87);

    const noCoverage = normalizeTrackDetail(detail);
    expect(noCoverage.analysis.elevation_coverage).toBeNull();
  });

  it('converts each climb in the array', () => {
    const out = normalizeTrackDetail(detail);
    expect(out.climbs).toHaveLength(1);
    expect(out.climbs[0].gain_m).toBe(200);
    expect(out.climbs[0].distance_m).toBe(2000);
    expect(out.climbs[0].is_king_climb).toBe(true);
  });

  it('converts each risk zone', () => {
    const out = normalizeTrackDetail(detail);
    expect(out.risk_zones).toHaveLength(1);
    expect(out.risk_zones[0].category).toBe('steep');
    expect(out.risk_zones[0].severity).toBe(0.9);
  });

  it('converts uploaded_at to a Date', () => {
    const out = normalizeTrackDetail(detail);
    expect(out.track.uploaded_at).toBeInstanceOf(Date);
    expect(out.track.uploaded_at.toISOString()).toBe('2025-09-15T08:00:00.000Z');
  });
});
