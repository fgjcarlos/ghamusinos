import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import type { Workout } from '../../lib/api/workout-types';
import { WorkoutsContainer } from './WorkoutsContainer';
import * as api from '../../lib/api/workouts';

vi.mock('../../lib/api/workouts', () => ({
  listWorkouts: vi.fn(),
  createWorkout: vi.fn(),
  updateWorkout: vi.fn(),
  deleteWorkout: vi.fn(),
}));

const workout = (id: string): Workout => ({
  id,
  scheduled_on: '2026-12-01',
  title: `Entreno ${id}`,
  notes: '',
  workout_type: 'rodaje',
  target_duration_sec: null,
  target_distance_m: null,
  target_elevation_gain_m: null,
  target_intensity: null,
});
const page = (data: Workout[], has_next: boolean) => ({
  data,
  total: data.length,
  limit: 20,
  offset: 0,
  has_next,
});

beforeEach(() => {
  vi.clearAllMocks();
  vi.stubEnv('VITE_AUTH_TOKEN', 'token');
  vi.mocked(api.listWorkouts).mockReset().mockResolvedValue(page([], false));
  vi.mocked(api.deleteWorkout).mockReset().mockResolvedValue();
  vi.stubGlobal(
    'confirm',
    vi.fn(() => true),
  );
});

describe('workout retry serialization', () => {
  it('hides stale pagination during retry and resumes from the refreshed first-page offset', async () => {
    const first = Array.from({ length: 20 }, (_, index) =>
      workout(`r${String(index).padStart(2, '0')}`),
    );
    const retryPage = Array.from({ length: 20 }, (_, index) =>
      workout(`r${String(index + 1).padStart(2, '0')}`),
    );
    const nextPage = Array.from({ length: 20 }, (_, index) =>
      workout(`r${String(index + 21).padStart(2, '0')}`),
    );
    let resolveRetry!: (value: ReturnType<typeof page>) => void;
    vi.mocked(api.listWorkouts)
      .mockReset()
      .mockResolvedValueOnce(page(first, true))
      .mockRejectedValueOnce(new Error('refresh failed'))
      .mockReturnValueOnce(new Promise((resolve) => (resolveRetry = resolve)))
      .mockResolvedValueOnce(page(nextPage, false));

    render(<WorkoutsContainer />);
    await screen.findByRole('button', { name: 'Borrar Entreno r00' });
    fireEvent.click(screen.getByRole('button', { name: 'Borrar Entreno r00' }));
    fireEvent.click(await screen.findByRole('button', { name: 'Reintentar' }));
    await waitFor(() => expect(api.listWorkouts).toHaveBeenCalledTimes(3));

    expect(screen.queryByRole('button', { name: 'Cargar más' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Borrar Entreno r01' })).not.toBeInTheDocument();
    resolveRetry(page(retryPage, true));
    await screen.findByRole('button', { name: 'Cargar más' });
    fireEvent.click(screen.getByRole('button', { name: 'Cargar más' }));
    await screen.findByRole('button', { name: 'Borrar Entreno r40' });
    expect(
      (api.listWorkouts as unknown as ReturnType<typeof vi.fn>).mock.calls.map(
        (call: unknown[]) => call[4],
      ),
    ).toEqual([0, 0, 0, 20]);
    expect(document.querySelectorAll('[aria-labelledby^="day-"] > ul > li')).toHaveLength(40);
  });

  it('prevents a second delete during retry and settles to empty after that retry resolves', async () => {
    let resolveRetry!: (value: ReturnType<typeof page>) => void;
    vi.mocked(api.listWorkouts)
      .mockReset()
      .mockResolvedValueOnce(page([workout('A'), workout('B')], false))
      .mockRejectedValueOnce(new Error('refresh failed'))
      .mockReturnValueOnce(new Promise((resolve) => (resolveRetry = resolve)));

    render(<WorkoutsContainer />);
    await screen.findByRole('button', { name: 'Borrar Entreno A' });
    fireEvent.click(screen.getByRole('button', { name: 'Borrar Entreno A' }));
    fireEvent.click(await screen.findByRole('button', { name: 'Reintentar' }));
    await waitFor(() => expect(api.listWorkouts).toHaveBeenCalledTimes(3));

    expect(screen.queryByRole('button', { name: 'Borrar Entreno B' })).not.toBeInTheDocument();
    expect(api.deleteWorkout).toHaveBeenCalledTimes(1);
    resolveRetry(page([], false));
    expect(
      await screen.findByText('No hay entrenos planificados en este periodo.'),
    ).toBeInTheDocument();
    expect(screen.queryByRole('status', { name: 'Cargando entrenos' })).not.toBeInTheDocument();
    expect(api.deleteWorkout).toHaveBeenCalledTimes(1);
  });
});
