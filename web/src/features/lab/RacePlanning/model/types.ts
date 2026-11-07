/**
 * Shared types for the phase 1.6 race-planning model.
 *
 * The model is pure TypeScript and never touches the network. Coordinates are
 * named fields (`lat`, `lon`, `ele`) so they can be fed straight from the GPX
 * detail wire shape without a positional convention.
 */

export interface TrackPoint {
  lat: number;
  lon: number;
  ele: number | null;
}

export interface RacePacePreset {
  id: string;
  label: string;
  paceMinPerKm: number;
}

export interface KmMarker {
  km: number;
  index: number;
  lat: number;
  lon: number;
  cumulativeGainM: number;
}

export interface NutritionPoint {
  km: number;
  index: number;
  lat: number;
  lon: number;
  kind: 'scheduled' | 'recovery';
}

export interface SplitRow {
  km: number;
  splitTimeMin: number;
  cumulativeTimeMin: number;
  gainM: number;
  paceMinPerKm: number;
}

export interface CutoffInput {
  km: number;
  timeLimitMin: number;
  label?: string;
}

export interface CutoffResult extends CutoffInput {
  estimatedMin: number;
  isSafe: boolean;
  marginMin: number;
  requiredPaceMinPerKm: number;
}

export interface SolarSegment {
  startKm: number;
  endKm: number;
  status: 'sun' | 'shade' | 'night';
  intensity: number;
  aspectDeg: number;
  slopePercent: number;
}

export interface GearItem {
  id: string;
  name: string;
  category: 'mandatory' | 'recommended';
  reason: string;
}

export interface GearContext {
  durationHours: number;
  dPlusM: number;
  maxAltitudeM: number;
  minTemperatureC?: number;
  maxWindKmh?: number;
  rainy?: boolean;
}

export interface GearChecklist {
  mandatory: GearItem[];
  recommended: GearItem[];
}

export interface PlanVsRealRow {
  km: number;
  plannedSplitMin: number;
  realSplitMin: number;
  diffMin: number;
  plannedCumulativeMin: number;
  realCumulativeMin: number;
  fatigueIndex: number;
}
