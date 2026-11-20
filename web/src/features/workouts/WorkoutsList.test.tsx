import { describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { WorkoutsList } from './WorkoutsList';
import type { Workout } from '../../lib/api/workout-types';

const session = (id: string): Workout => ({
  id,
  scheduled_on: '2026-11-17',
  title: id,
  notes: '',
  workout_type: 'tirada_larga',
  target_duration_sec: null,
  target_distance_m: 0,
  target_elevation_gain_m: null,
  target_intensity: 'z2',
});

describe('WorkoutsList', () => {
  it('renders one civil-day heading for same-day items and translated type/targets', () => {
    render(
      <WorkoutsList
        workouts={[session('two'), session('one')]}
        onEdit={vi.fn()}
        onDelete={vi.fn()}
        busy={false}
        hasNext={false}
        loadingMore={false}
        onLoadMore={vi.fn()}
      />,
    );
    expect(
      screen.getAllByRole('heading', { name: /martes, 17 de noviembre de 2026/i }),
    ).toHaveLength(1);
    expect(screen.getAllByText('Tirada larga')).toHaveLength(2);
    expect(screen.getAllByText(/Distancia: 0 m/)).toHaveLength(2);
  });

  it('disables conflicting controls while a mutation is pending', () => {
    render(
      <WorkoutsList
        workouts={[session('one')]}
        onEdit={vi.fn()}
        onDelete={vi.fn()}
        busy
        hasNext
        onLoadMore={vi.fn()}
        loadingMore={false}
      />,
    );
    expect(screen.getByRole('button', { name: 'Editar one' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Borrar one' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Cargar más' })).toBeDisabled();
  });
});
