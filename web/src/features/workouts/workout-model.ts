import type { Workout } from '../../lib/api/workout-types';

export interface WorkoutValues {
  scheduled_on: string;
  title: string;
  notes: string;
  workout_type: string;
  target_duration_sec: string;
  target_distance_m: string;
  target_elevation_gain_m: string;
  target_intensity: string;
}

const workoutTypes = new Set([
  'rodaje',
  'tirada_larga',
  'series',
  'cuestas',
  'tempo',
  'fartlek',
  'fuerza',
  'recuperacion',
  'otro',
]);
const maxTarget = 2147483647;
const utf8Length = (value: string) => new TextEncoder().encode(value).length;

export function validateWorkoutValues(values: WorkoutValues): Record<string, string> {
  const errors: Record<string, string> = {};
  const { scheduled_on: date, title, notes, workout_type: type } = values;
  if (
    !/^\d{4}-\d{2}-\d{2}$/.test(date) ||
    Number.isNaN(Date.parse(`${date}T00:00:00Z`)) ||
    new Date(`${date}T00:00:00Z`).toISOString().slice(0, 10) !== date
  )
    errors.scheduled_on = 'Introduce una fecha válida.';
  if (!title) errors.title = 'El título es obligatorio.';
  if (utf8Length(title) > 200) errors.title = 'Máximo 200 bytes.';
  if (utf8Length(notes) > 2000) errors.notes = 'Máximo 2000 bytes.';
  if (!workoutTypes.has(type)) errors.workout_type = 'Selecciona un tipo válido.';
  for (const key of [
    'target_duration_sec',
    'target_distance_m',
    'target_elevation_gain_m',
  ] as const) {
    const value = values[key];
    if (value && (!/^\d+$/.test(value) || Number(value) > maxTarget))
      errors[key] = 'Introduce un entero entre 0 y 2147483647.';
  }
  if (values.target_intensity && !/^z[1-5]$/.test(values.target_intensity))
    errors.target_intensity = 'Selecciona una intensidad válida.';
  return errors;
}

export function getWorkoutWindow(now: Date): { from: string; to: string } {
  const from = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-${String(now.getDate()).padStart(2, '0')}`;
  const year = now.getFullYear() + 1;
  const month = now.getMonth();
  const day = Math.min(now.getDate(), new Date(year, month + 1, 0).getDate());
  const to = `${year}-${String(month + 1).padStart(2, '0')}-${String(day).padStart(2, '0')}`;
  return { from, to };
}

export function groupWorkouts(workouts: Workout[]): Array<{ date: string; items: Workout[] }> {
  const sorted = [...workouts].sort(
    (a, b) => a.scheduled_on.localeCompare(b.scheduled_on) || a.id.localeCompare(b.id),
  );
  const groups: Array<{ date: string; items: Workout[] }> = [];
  for (const workout of sorted) {
    const last = groups.length ? groups[groups.length - 1] : undefined;
    if (last?.date === workout.scheduled_on) last.items.push(workout);
    else groups.push({ date: workout.scheduled_on, items: [workout] });
  }
  return groups;
}

export function workoutValuesFromForm(form: HTMLFormElement): WorkoutValues {
  const data = new FormData(form);
  return Object.fromEntries(
    [
      'scheduled_on',
      'title',
      'notes',
      'workout_type',
      'target_duration_sec',
      'target_distance_m',
      'target_elevation_gain_m',
      'target_intensity',
    ].map((key) => [key, String(data.get(key) ?? '')]),
  ) as unknown as WorkoutValues;
}
