import { describe, expect, it } from 'vitest';
import { getWorkoutWindow, groupWorkouts, validateWorkoutValues } from './workout-model';
import type { Workout } from '../../lib/api/workout-types';

const base: Workout = {
  id: 'b',
  scheduled_on: '2026-11-17',
  title: 'B',
  notes: '',
  workout_type: 'rodaje',
  target_duration_sec: null,
  target_distance_m: null,
  target_elevation_gain_m: null,
  target_intensity: null,
};

describe('workout model', () => {
  it('clamps a leap-year window to the final valid day', () => {
    expect(getWorkoutWindow(new Date(2024, 1, 29))).toEqual({
      from: '2024-02-29',
      to: '2025-02-28',
    });
  });
  it('groups by civil day, sorts deterministically, and omits duplicate day headers', () => {
    const grouped = groupWorkouts([
      { ...base, id: 'z' },
      { ...base, id: 'a' },
      { ...base, id: 'c', scheduled_on: '2026-11-18' },
    ]);
    expect(grouped.map((group) => [group.date, group.items.map((item) => item.id)])).toEqual([
      ['2026-11-17', ['a', 'z']],
      ['2026-11-18', ['c']],
    ]);
  });
  it('matches backend byte limits and rejects invalid numerics and civil dates', () => {
    expect(
      validateWorkoutValues({
        scheduled_on: '2026-02-31',
        title: '',
        notes: 'á'.repeat(1001),
        workout_type: 'rodaje',
        target_duration_sec: '-1',
        target_distance_m: '1.5',
        target_elevation_gain_m: '2147483648',
        target_intensity: '',
      }),
    ).toMatchObject({
      scheduled_on: expect.any(String),
      title: expect.any(String),
      notes: expect.any(String),
      target_duration_sec: expect.any(String),
      target_distance_m: expect.any(String),
      target_elevation_gain_m: expect.any(String),
    });
    expect(
      validateWorkoutValues({
        scheduled_on: '2026-02-28',
        title: 'á'.repeat(101),
        notes: 'x',
        workout_type: 'rodaje',
        target_duration_sec: '',
        target_distance_m: '',
        target_elevation_gain_m: '',
        target_intensity: '',
      }).title,
    ).toBe('Máximo 200 bytes.');
    expect(
      validateWorkoutValues({
        scheduled_on: '2026-02-28',
        title: 'á'.repeat(100),
        notes: 'x',
        workout_type: 'rodaje',
        target_duration_sec: '0',
        target_distance_m: '',
        target_elevation_gain_m: '',
        target_intensity: '',
      }),
    ).toEqual({});
  });
});
