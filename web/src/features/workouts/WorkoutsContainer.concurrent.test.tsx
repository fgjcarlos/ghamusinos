import { beforeEach, describe, expect, it, vi } from 'vitest';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { WorkoutsContainer } from './WorkoutsContainer';
import type { Workout } from '../../lib/api/workout-types';
import * as api from '../../lib/api/workouts';
import { ApiError } from '../../lib/api/types';

vi.mock('../../lib/api/workouts', () => ({
  listWorkouts: vi.fn(),
  createWorkout: vi.fn(),
  updateWorkout: vi.fn(),
  deleteWorkout: vi.fn(),
}));
const session = (id: string, title = `Plan ${id}`): Workout => ({
  id,
  scheduled_on: '2026-12-01',
  title,
  notes: '',
  workout_type: 'rodaje',
  target_duration_sec: null,
  target_distance_m: null,
  target_elevation_gain_m: null,
  target_intensity: null,
});
const page = (data: Workout[], has_next = false) => ({
  data,
  total: 40,
  limit: 20,
  offset: 0,
  has_next,
});
const input = (name: string) =>
  document.querySelector<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>(
    `[name="${name}"]`,
  )!;

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.listWorkouts)
    .mockReset()
    .mockResolvedValue(page([session('a', 'Rodaje suave')]));
  vi.mocked(api.createWorkout).mockReset();
  vi.mocked(api.updateWorkout).mockReset();
  vi.mocked(api.deleteWorkout).mockReset();
  vi.stubEnv('VITE_AUTH_TOKEN', 'token');
  vi.stubGlobal(
    'confirm',
    vi.fn(() => true),
  );
});

describe('WorkoutsContainer', () => {
  it('does not lose or duplicate records when loading another page after a create', async () => {
    const firstPage = Array.from({ length: 20 }, (_, index) =>
      session(String(index).padStart(2, '0')),
    );
    const nextPage = Array.from({ length: 20 }, (_, index) =>
      session(String(index + 20).padStart(2, '0')),
    );
    const created = session('created', 'Nuevo entreno');
    vi.mocked(api.listWorkouts)
      .mockReset()
      .mockResolvedValueOnce(page(firstPage, true))
      .mockResolvedValueOnce(page(firstPage, true))
      .mockResolvedValueOnce(page(nextPage, false));
    vi.mocked(api.createWorkout).mockResolvedValue(created);
    render(<WorkoutsContainer />);
    await screen.findByText('Plan 00');
    fireEvent.click(screen.getByRole('button', { name: 'Nuevo entreno' }));
    fireEvent.change(input('scheduled_on'), { target: { value: '2026-12-01' } });
    fireEvent.change(input('title'), { target: { value: 'Nuevo entreno' } });
    fireEvent.click(screen.getByRole('button', { name: 'Guardar' }));
    await screen.findByRole('heading', { name: 'Nuevo entreno' });
    fireEvent.click(screen.getByRole('button', { name: 'Cargar más' }));
    await screen.findByText('Plan 39');
    expect(document.querySelectorAll('[aria-labelledby^="day-"] li')).toHaveLength(41);
    expect(
      (api.listWorkouts as unknown as ReturnType<typeof vi.fn>).mock.calls.map(
        (call: unknown[]) => call[4],
      ),
    ).toEqual([0, 0, 20]);
  });

  it('preserves form values on save failure and disables conflicting controls while pending', async () => {
    let resolveCreate!: (value: Workout) => void;
    vi.mocked(api.createWorkout).mockReturnValueOnce(
      new Promise((resolve) => {
        resolveCreate = resolve;
      }),
    );
    render(<WorkoutsContainer />);
    await screen.findByText('Rodaje suave');
    fireEvent.click(screen.getByRole('button', { name: 'Nuevo entreno' }));
    fireEvent.change(input('scheduled_on'), { target: { value: '2026-12-01' } });
    fireEvent.change(input('title'), { target: { value: 'En curso' } });
    const form = document.querySelector('form')!;
    act(() => {
      fireEvent.submit(form);
      fireEvent.submit(form);
    });
    expect(screen.getByRole('button', { name: 'Cancelar' })).toBeDisabled();
    expect(input('title')).toBeDisabled();
    expect(api.createWorkout).toHaveBeenCalledTimes(1);
    resolveCreate(session('new', 'En curso'));
    await waitFor(() =>
      expect(screen.queryByRole('button', { name: 'Guardar' })).not.toBeInTheDocument(),
    );
  });

  it('keeps form values after save error and shows its request ID', async () => {
    vi.mocked(api.createWorkout).mockRejectedValueOnce(
      new ApiError(500, 'No se pudo guardar', undefined, 'req-save'),
    );
    render(<WorkoutsContainer />);
    await screen.findByText('Rodaje suave');
    fireEvent.click(screen.getByRole('button', { name: 'Nuevo entreno' }));
    fireEvent.change(input('scheduled_on'), { target: { value: '2026-12-01' } });
    fireEvent.change(input('title'), { target: { value: 'Conservar' } });
    fireEvent.click(screen.getByRole('button', { name: 'Guardar' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('req-save');
    expect(input('title')).toHaveValue('Conservar');
    expect(screen.getByRole('button', { name: 'Guardar' })).toBeInTheDocument();
  });
});
