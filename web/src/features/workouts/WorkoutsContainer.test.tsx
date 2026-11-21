import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';
import { WorkoutsContainer } from './WorkoutsContainer';
import { ApiError } from '../../lib/api/types';
import type { Workout } from '../../lib/api/workout-types';
import * as api from '../../lib/api/workouts';

vi.mock('../../lib/api/workouts', () => ({
  listWorkouts: vi.fn(),
  createWorkout: vi.fn(),
  updateWorkout: vi.fn(),
  deleteWorkout: vi.fn(),
}));
const first: Workout = {
  id: 'a',
  scheduled_on: '2026-12-01',
  title: 'Rodaje suave',
  notes: '',
  workout_type: 'rodaje',
  target_duration_sec: 0,
  target_distance_m: null,
  target_elevation_gain_m: null,
  target_intensity: null,
  status: 'planned',
  activity_id: null,
};
const page = (data: Workout[], has_next = false, offset = 0) => ({
  data,
  total: 40,
  limit: 20,
  offset,
  has_next,
});
const input = (name: string) =>
  document.querySelector<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>(
    `[name="${name}"]`,
  )!;

beforeEach(() => {
  vi.clearAllMocks();
  vi.stubEnv('VITE_AUTH_TOKEN', 'token');
  vi.mocked(api.listWorkouts).mockResolvedValue(page([first]));
  vi.stubGlobal(
    'confirm',
    vi.fn(() => true),
  );
});

describe('WorkoutsContainer', () => {
  it('renders civil-date groups and zero targets', async () => {
    render(<WorkoutsContainer />);
    expect(await screen.findByText('Rodaje suave')).toBeInTheDocument();
    expect(screen.getByText(/Duración: 0 s/)).toBeInTheDocument();
    expect(
      screen.getByRole('heading', { name: /martes, 1 de diciembre de 2026/i }),
    ).toBeInTheDocument();
  });

  it('validates required fields and invalid civil dates', async () => {
    render(<WorkoutsContainer />);
    await screen.findByText('Rodaje suave');
    fireEvent.click(screen.getByRole('button', { name: 'Nuevo entreno' }));
    fireEvent.change(input('scheduled_on'), { target: { value: '2026-02-31' } });
    fireEvent.click(screen.getByRole('button', { name: 'Guardar' }));
    expect(await screen.findByText('Introduce una fecha válida.')).toBeInTheDocument();
    expect(screen.getByText('El título es obligatorio.')).toBeInTheDocument();
    expect(api.createWorkout).not.toHaveBeenCalled();
  });

  it('creates and edits through API, then refreshes page contents', async () => {
    const created = { ...first, id: 'b', title: 'Series', scheduled_on: '2026-12-02' };
    const updated = { ...created, title: 'Series cortas' };
    vi.mocked(api.listWorkouts)
      .mockResolvedValueOnce(page([first]))
      .mockResolvedValueOnce(page([first]))
      .mockResolvedValueOnce(page([first]));
    vi.mocked(api.createWorkout).mockResolvedValue(created);
    vi.mocked(api.updateWorkout).mockResolvedValue(updated);
    render(<WorkoutsContainer />);
    await screen.findByText('Rodaje suave');
    fireEvent.click(screen.getByRole('button', { name: 'Nuevo entreno' }));
    fireEvent.change(input('scheduled_on'), { target: { value: '2026-12-02' } });
    fireEvent.change(input('title'), { target: { value: 'Series' } });
    fireEvent.click(screen.getByRole('button', { name: 'Guardar' }));
    expect(await screen.findByText('Series')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Editar Series' }));
    fireEvent.change(input('title'), { target: { value: 'Series cortas' } });
    fireEvent.click(screen.getByRole('button', { name: 'Guardar' }));
    expect(await screen.findByText('Series cortas')).toBeInTheDocument();
    expect(api.updateWorkout).toHaveBeenCalledWith(
      'token',
      'b',
      expect.objectContaining({ title: 'Series cortas' }),
    );
  });

  it('cancels deletion or deletes and reloads on confirmation', async () => {
    render(<WorkoutsContainer />);
    await screen.findByText('Rodaje suave');
    vi.stubGlobal(
      'confirm',
      vi.fn(() => false),
    );
    fireEvent.click(screen.getByRole('button', { name: 'Borrar Rodaje suave' }));
    expect(api.deleteWorkout).not.toHaveBeenCalled();
    vi.stubGlobal(
      'confirm',
      vi.fn(() => true),
    );
    vi.mocked(api.deleteWorkout).mockResolvedValue();
    vi.mocked(api.listWorkouts).mockResolvedValueOnce(page([]));
    fireEvent.click(screen.getByRole('button', { name: 'Borrar Rodaje suave' }));
    expect(await screen.findByText(/No hay entrenos planificados/)).toBeInTheDocument();
    expect(api.deleteWorkout).toHaveBeenCalledWith('token', 'a');
  });

  it('shows delete failures and leaves the session in the list', async () => {
    vi.mocked(api.deleteWorkout).mockRejectedValueOnce(
      new ApiError(500, 'No se pudo borrar', undefined, 'req-delete'),
    );
    render(<WorkoutsContainer />);
    await screen.findByText('Rodaje suave');
    fireEvent.click(screen.getByRole('button', { name: 'Borrar Rodaje suave' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('req-delete');
    expect(screen.getByText('Rodaje suave')).toBeInTheDocument();
  });

  it('shows a loading skeleton with busy semantics until the initial request resolves', async () => {
    let resolvePage!: (value: ReturnType<typeof page>) => void;
    vi.mocked(api.listWorkouts).mockReturnValueOnce(
      new Promise((resolve) => {
        resolvePage = resolve;
      }),
    );
    render(<WorkoutsContainer />);
    expect(screen.getByRole('status', { name: 'Cargando entrenos' })).toHaveAttribute(
      'aria-busy',
      'true',
    );
    resolvePage(page([first]));
    expect(await screen.findByText('Rodaje suave')).toBeInTheDocument();
  });

  it('shows request ID on list failure and retries instead of showing empty state', async () => {
    vi.mocked(api.listWorkouts).mockRejectedValueOnce(
      new ApiError(500, 'backend', undefined, 'req-500'),
    );
    render(<WorkoutsContainer />);
    expect(await screen.findByRole('alert')).toHaveTextContent('req-500');
    expect(screen.queryByText(/No hay entrenos planificados/)).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Reintentar' }));
    expect(await screen.findByText('Rodaje suave')).toBeInTheDocument();
  });

  it('retries a failed load-more at the same offset and serializes repeated clicks', async () => {
    const firstPage = Array.from({ length: 20 }, (_, index) => ({
      ...first,
      id: `id-${index}`,
      title: `Plan ${index}`,
    }));
    const extra = { ...first, id: 'extra', title: 'Plan extra' };
    vi.mocked(api.listWorkouts)
      .mockReset()
      .mockResolvedValueOnce(page(firstPage, true))
      .mockRejectedValueOnce(new Error('offline'))
      .mockResolvedValueOnce(page([extra], false, 20));
    render(<WorkoutsContainer />);
    await screen.findByText('Plan 0');
    fireEvent.click(screen.getByRole('button', { name: 'Cargar más' }));
    const loadingButton = screen.getByRole('button', { name: 'Cargando…' });
    expect(loadingButton).toBeDisabled();
    fireEvent.click(loadingButton);
    expect(await screen.findByRole('alert')).toHaveTextContent('offline');
    fireEvent.click(screen.getByRole('button', { name: 'Cargar más' }));
    expect(await screen.findByText('Plan extra')).toBeInTheDocument();
    expect(
      (api.listWorkouts as unknown as ReturnType<typeof vi.fn>).mock.calls.map(
        (call: unknown[]) => call[4],
      ),
    ).toEqual([0, 20, 20]);
  });

  it('does not request pages with a missing auth token', async () => {
    vi.stubEnv('VITE_AUTH_TOKEN', '');
    render(<WorkoutsContainer />);
    expect(await screen.findByRole('alert')).toHaveTextContent('VITE_AUTH_TOKEN');
    expect(api.listWorkouts).not.toHaveBeenCalled();
  });
});
