export type WorkoutType =
  | 'rodaje'
  | 'tirada_larga'
  | 'series'
  | 'cuestas'
  | 'tempo'
  | 'fartlek'
  | 'fuerza'
  | 'recuperacion'
  | 'otro';
export type WorkoutIntensity = 'z1' | 'z2' | 'z3' | 'z4' | 'z5';
export interface Workout {
  id: string;
  scheduled_on: string;
  title: string;
  notes: string;
  workout_type: WorkoutType;
  target_duration_sec: number | null;
  target_distance_m: number | null;
  target_elevation_gain_m: number | null;
  target_intensity: WorkoutIntensity | null;
}
export type WorkoutInput = Omit<Workout, 'id' | 'notes'> & { notes?: string };
export interface PaginatedWorkouts {
  data: Workout[];
  total: number;
  limit: number;
  offset: number;
  has_next: boolean;
}
