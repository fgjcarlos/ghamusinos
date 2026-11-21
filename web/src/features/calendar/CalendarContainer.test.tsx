// TDD contract for the interactive calendar (#313, T3): week/month
// views in the user's zone, native drag & drop with keyboard/touch
// alternatives for every action, status selection, create-from-day
// reusing WorkoutForm, full pagination of the visible range and no
// stale requests across range changes.

import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { CalendarContainer } from './CalendarContainer';
import { createDeferred } from '../../test/deferred';
import {
  createWorkout,
  linkWorkoutActivity,
  listWorkouts,
  unlinkWorkoutActivity,
  updateWorkout,
} from '../../lib/api/workouts';
import { listCalendarActivities, loadCalendarComparison } from '../../lib/api/calendar';
import { getPreferences } from '../../lib/api/profile';
import { ApiError } from '../../lib/api/types';
import type { Workout } from '../../lib/api/workout-types';

vi.mock('../../lib/api/workouts', () => ({
  listWorkouts: vi.fn(),
  updateWorkout: vi.fn(),
  createWorkout: vi.fn(),
  linkWorkoutActivity: vi.fn(),
  unlinkWorkoutActivity: vi.fn(),
}));
vi.mock('../../lib/api/calendar', () => ({
  listCalendarActivities: vi.fn(),
  loadCalendarComparison: vi.fn(),
}));
vi.mock('../../lib/api/profile', () => ({ getPreferences: vi.fn() }));

const now = new Date('2026-11-25T12:00:00Z'); // hoy en Madrid: 2026-11-25

const workout = (
  over: Partial<Workout> & Pick<Workout, 'id' | 'scheduled_on' | 'title'>,
): Workout => ({
  notes: '',
  workout_type: 'rodaje',
  target_duration_sec: null,
  target_distance_m: null,
  target_elevation_gain_m: null,
  target_intensity: null,
  status: 'planned',
  activity_id: null,
  ...over,
});
const rodazo = workout({ id: 's1', scheduled_on: '2026-11-20', title: 'Rodazo' });
const linked = workout({
  id: 's2',
  scheduled_on: '2026-11-27',
  title: 'Tirada larga',
  status: 'completed',
  activity_id: 'a2',
});
const trail = { id: 'a1', name: 'Trail matinal', scheduled_on: '2026-11-21', linked: false };
const series = { id: 'a2', name: 'Series park', scheduled_on: '2026-11-27', linked: true };

function happy({ sessions = [rodazo, linked] as Workout[], activities = [trail, series] } = {}) {
  vi.mocked(listWorkouts).mockImplementation(async (_t, from, to, limit = 100, offset = 0) => ({
    data: sessions.filter((s) => s.scheduled_on >= from && s.scheduled_on <= to),
    total: sessions.length,
    limit,
    offset,
    has_next: false,
  }));
  vi.mocked(listCalendarActivities).mockResolvedValue({ activities });
  vi.mocked(loadCalendarComparison).mockResolvedValue({ periods: [] });
  vi.mocked(getPreferences).mockResolvedValue({
    hr_max: null,
    lthr: null,
    ftp: null,
    level: null,
    timezone: 'Europe/Madrid',
    ai_enabled: false,
  });
}

const SESSION_MIME = 'application/x-ghamusinos-session';
const ACTIVITY_MIME = 'application/x-ghamusinos-activity';
const dropPayload = (mime: string, id: string) => ({
  dataTransfer: {
    types: [mime],
    getData: (type: string) => (type === mime ? id : ''),
  },
});
const dragPayload = () => ({ dataTransfer: { setData: vi.fn(), effectAllowed: '' } });
const cell = (date: string) => document.querySelector(`[data-date="${date}"]`)!;

beforeEach(() => {
  vi.stubEnv('VITE_AUTH_TOKEN', 'token');
  vi.clearAllMocks();
});

describe('CalendarContainer', () => {
  it('renders the month grid with sessions and activities of the padded range', async () => {
    happy();
    render(<CalendarContainer now={now} />);
    expect(await screen.findByText('Rodazo')).toBeInTheDocument();
    // La malla de noviembre va del lunes 26-10 al domingo 6-12.
    expect(vi.mocked(listWorkouts).mock.calls[0]?.slice(1, 3)).toEqual([
      '2026-10-26',
      '2026-12-06',
    ]);
    expect(vi.mocked(listCalendarActivities).mock.calls[0]?.slice(1, 3)).toEqual([
      '2026-10-26',
      '2026-12-06',
    ]);
    expect(screen.getByText(/Trail matinal · libre/)).toBeInTheDocument();
    expect(screen.getByText(/Series park · vinculada/)).toBeInTheDocument();
    // La bandeja del periodo solo ofrece las actividades sin vincular.
    expect(screen.getByText(/Trail matinal · 2026-11-21/)).toBeInTheDocument();
    expect(screen.queryByText(/Series park · 2026-11-27/)).not.toBeInTheDocument();
    expect(cell('2026-11-25').querySelector('[aria-current="date"]')).not.toBeNull();
    expect(screen.getByRole('heading', { name: 'Noviembre de 2026' })).toBeInTheDocument();
  });

  it('fetches every page of sessions in the visible range', async () => {
    happy();
    const page = (offset: number) => ({
      data: offset === 0 ? [rodazo] : [linked],
      total: 2,
      limit: 100,
      offset,
      has_next: offset === 0,
    });
    vi.mocked(listWorkouts).mockImplementation(async (_t, _f, _to, _l, offset = 0) => page(offset));
    render(<CalendarContainer now={now} />);
    expect(await screen.findByText('Tirada larga')).toBeInTheDocument();
    expect(screen.getByText('Rodazo')).toBeInTheDocument();
    expect(vi.mocked(listWorkouts).mock.calls.length).toBeGreaterThanOrEqual(2);
  });

  it('switches to a seven-day week view and refetches its range', async () => {
    happy();
    render(<CalendarContainer now={now} />);
    await screen.findByText('Rodazo');
    fireEvent.click(screen.getByRole('button', { name: 'Semana' }));
    expect(
      await screen.findByRole('heading', { name: '23 – 29 de noviembre de 2026' }),
    ).toBeInTheDocument();
    expect(vi.mocked(listWorkouts).mock.calls.slice(-1)[0]?.slice(1, 3)).toEqual([
      '2026-11-23',
      '2026-11-29',
    ]);
    expect(document.querySelectorAll('[data-date]')).toHaveLength(7);
    expect(
      screen.getByRole('heading', { name: '23 – 29 de noviembre de 2026' }),
    ).toBeInTheDocument();
  });

  it('navigates previous, next and back to today', async () => {
    happy();
    render(<CalendarContainer now={now} />);
    await screen.findByText('Rodazo');
    fireEvent.click(screen.getByRole('button', { name: 'Siguiente' }));
    expect(await screen.findByRole('heading', { name: 'Diciembre de 2026' })).toBeInTheDocument();
    expect(vi.mocked(listWorkouts).mock.calls.slice(-1)[0]?.slice(1, 3)).toEqual([
      '2026-11-30',
      '2027-01-10',
    ]);
    fireEvent.click(screen.getByRole('button', { name: 'Anterior' }));
    expect(await screen.findByRole('heading', { name: 'Noviembre de 2026' })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Siguiente' }));
    await screen.findByRole('heading', { name: 'Diciembre de 2026' });
    fireEvent.click(screen.getByRole('button', { name: 'Hoy' }));
    expect(await screen.findByRole('heading', { name: 'Noviembre de 2026' })).toBeInTheDocument();
  });

  it('moves a session by native drag & drop and reloads', async () => {
    happy();
    vi.mocked(updateWorkout).mockResolvedValue(rodazo);
    render(<CalendarContainer now={now} />);
    await screen.findByText('Rodazo');
    const loads = vi.mocked(listWorkouts).mock.calls.length;
    fireEvent.dragStart(screen.getByText('Rodazo').closest('li')!, dragPayload());
    fireEvent.drop(
      screen.getByText('Tirada larga').closest('li')!,
      dropPayload(SESSION_MIME, 's1'),
    );
    await waitFor(() =>
      expect(vi.mocked(updateWorkout)).toHaveBeenCalledWith('token', 's1', {
        scheduled_on: '2026-11-27',
      }),
    );
    await waitFor(() => expect(vi.mocked(listWorkouts).mock.calls.length).toBeGreaterThan(loads));
  });

  it('moves a session with the keyboard/touch date input alternative', async () => {
    happy();
    vi.mocked(updateWorkout).mockResolvedValue(rodazo);
    render(<CalendarContainer now={now} />);
    await screen.findByText('Rodazo');
    fireEvent.change(screen.getByLabelText('Mover Rodazo a otra fecha'), {
      target: { value: '2026-11-24' },
    });
    await waitFor(() =>
      expect(vi.mocked(updateWorkout)).toHaveBeenCalledWith('token', 's1', {
        scheduled_on: '2026-11-24',
      }),
    );
  });

  it('marks the session status', async () => {
    happy();
    vi.mocked(updateWorkout).mockResolvedValue(rodazo);
    render(<CalendarContainer now={now} />);
    await screen.findByText('Rodazo');
    fireEvent.change(screen.getByLabelText('Estado de Rodazo'), {
      target: { value: 'completed' },
    });
    await waitFor(() =>
      expect(vi.mocked(updateWorkout)).toHaveBeenCalledWith('token', 's1', {
        status: 'completed',
      }),
    );
  });

  it('links an activity through the session selector', async () => {
    happy();
    vi.mocked(linkWorkoutActivity).mockResolvedValue(rodazo);
    render(<CalendarContainer now={now} />);
    await screen.findByText('Rodazo');
    fireEvent.change(screen.getByLabelText('Vincular actividad a Rodazo'), {
      target: { value: 'a1' },
    });
    await waitFor(() =>
      expect(vi.mocked(linkWorkoutActivity)).toHaveBeenCalledWith('token', 's1', 'a1'),
    );
    // El selector solo ofrece actividades sin vincular del periodo.
    const options = screen
      .getByLabelText('Vincular actividad a Rodazo')
      .querySelectorAll<HTMLOptionElement>('option[value]');
    expect([...options].map((o) => o.value).filter(Boolean)).toEqual(['a1']);
  });

  it('shows the server status after linking and reloading', async () => {
    const sessions = [rodazo];
    const activities = [trail];
    happy({ sessions, activities });
    vi.mocked(linkWorkoutActivity).mockImplementationOnce(async () => {
      sessions[0] = { ...rodazo, status: 'completed', activity_id: 'a1' };
      activities[0] = { ...trail, linked: true };
      return sessions[0];
    });
    render(<CalendarContainer now={now} />);
    await screen.findByText('Rodazo');
    fireEvent.change(screen.getByLabelText('Vincular actividad a Rodazo'), {
      target: { value: 'a1' },
    });
    await screen.findByRole('button', { name: 'Desvincular Rodazo' });
    expect(screen.getByLabelText('Estado de Rodazo')).toHaveValue('completed');
  });

  it('links by dragging a tray activity onto the session card without bubbling to the day cell', async () => {
    happy();
    vi.mocked(linkWorkoutActivity).mockResolvedValue(rodazo);
    render(<CalendarContainer now={now} />);
    await screen.findByText('Rodazo');
    fireEvent.dragStart(
      screen.getByText(/Trail matinal · 2026-11-21/).closest('li')!,
      dragPayload(),
    );
    fireEvent.drop(screen.getByText('Rodazo').closest('li')!, dropPayload(ACTIVITY_MIME, 'a1'));
    await waitFor(() =>
      expect(vi.mocked(linkWorkoutActivity)).toHaveBeenCalledWith('token', 's1', 'a1'),
    );
    expect(vi.mocked(updateWorkout)).not.toHaveBeenCalled();
  });

  it('unlinks the activity of a session', async () => {
    happy();
    vi.mocked(unlinkWorkoutActivity).mockResolvedValue(linked);
    render(<CalendarContainer now={now} />);
    await screen.findByText('Tirada larga');
    fireEvent.click(screen.getByRole('button', { name: 'Desvincular Tirada larga' }));
    await waitFor(() =>
      expect(vi.mocked(unlinkWorkoutActivity)).toHaveBeenCalledWith('token', 's2'),
    );
  });

  it('creates a session from an empty day reusing WorkoutForm', async () => {
    happy();
    vi.mocked(createWorkout).mockResolvedValue(rodazo);
    render(<CalendarContainer now={now} />);
    await screen.findByText('Rodazo');
    fireEvent.click(screen.getByRole('button', { name: 'Crear sesión el 2026-11-24' }));
    const date = await screen.findByLabelText('Fecha');
    expect(date).toHaveValue('2026-11-24');
    fireEvent.change(screen.getByLabelText('Título'), { target: { value: 'Medias series' } });
    fireEvent.click(screen.getByRole('button', { name: 'Guardar' }));
    await waitFor(() =>
      expect(vi.mocked(createWorkout)).toHaveBeenCalledWith('token', {
        scheduled_on: '2026-11-24',
        title: 'Medias series',
        workout_type: 'rodaje',
        notes: '',
        target_duration_sec: null,
        target_distance_m: null,
        target_elevation_gain_m: null,
        target_intensity: null,
      }),
    );
    await waitFor(() => expect(screen.queryByLabelText('Título')).not.toBeInTheDocument());
  });

  it('shows the loading state and a retryable error when the feed fails', async () => {
    happy();
    vi.mocked(listWorkouts).mockRejectedValueOnce(new Error('boom'));
    render(<CalendarContainer now={now} />);
    expect(screen.getByRole('status')).toHaveTextContent('Cargando calendario');
    expect(await screen.findByRole('alert')).toHaveTextContent('Error al cargar el calendario.');
    fireEvent.click(screen.getByRole('button', { name: 'Reintentar' }));
    expect(await screen.findByText('Rodazo')).toBeInTheDocument();
  });

  it('renders the empty state when the range has no sessions or activities', async () => {
    happy({ sessions: [], activities: [] });
    render(<CalendarContainer now={now} />);
    expect(
      await screen.findByText('No hay sesiones ni actividades en este periodo.'),
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Crear sesión el 2026-11-25' })).toBeInTheDocument();
  });

  it('ignores a stale load after the visible range changes', async () => {
    happy();
    const stale = workout({ id: 's9', scheduled_on: '2026-12-05', title: 'Futura' });
    happy();
    const first = createDeferred<{
      data: Workout[];
      total: number;
      limit: number;
      offset: number;
      has_next: boolean;
    }>();
    const second = createDeferred<{
      data: Workout[];
      total: number;
      limit: number;
      offset: number;
      has_next: boolean;
    }>();
    const slow = first.promise;
    const fast = second.promise;
    vi.mocked(listWorkouts)
      .mockImplementationOnce(() => slow)
      .mockImplementationOnce(() => fast);
    render(<CalendarContainer now={now} />);
    // Espera al ancla antes de navegar: los botones estan deshabilitados
    // hasta resolver la zona del usuario.
    expect(await screen.findByRole('heading', { name: 'Noviembre de 2026' })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Siguiente' })); // cambia el rango
    second.resolve({ data: [], total: 0, limit: 100, offset: 0, has_next: false });
    await waitFor(() =>
      expect(screen.getByRole('heading', { name: 'Diciembre de 2026' })).toBeInTheDocument(),
    );
    await waitFor(() => expect(screen.queryByRole('alert')).not.toBeInTheDocument());
    first.resolve({ data: [stale], total: 1, limit: 100, offset: 0, has_next: false });
    await waitFor(() => expect(screen.queryByText('Futura')).not.toBeInTheDocument());
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('anchors today on the user zone from preferences, not the browser zone', async () => {
    happy();
    const lateUTC = new Date('2026-11-30T23:30:00Z'); // en Madrid ya es 1 de diciembre
    render(<CalendarContainer now={lateUTC} />);
    await screen.findByRole('heading', { name: 'Diciembre de 2026' });
    await waitFor(() => expect(cell('2026-12-01')).toBeInTheDocument());
    expect(vi.mocked(listWorkouts).mock.calls[0]?.slice(1, 3)).toEqual([
      '2026-11-30',
      '2027-01-10',
    ]);
    expect(cell('2026-12-01').querySelector('[aria-current="date"]')).not.toBeNull();
  });

  it('falls back to UTC when preferences fail', async () => {
    happy();
    vi.mocked(getPreferences).mockRejectedValue(new Error('offline'));
    const lateUTC = new Date('2026-11-30T23:30:00Z'); // en UTC sigue siendo 30
    render(<CalendarContainer now={lateUTC} />);
    await screen.findByRole('heading', { name: 'Noviembre de 2026' });
    await waitFor(() => expect(cell('2026-11-30')).toBeInTheDocument());
    expect(cell('2026-11-30').querySelector('[aria-current="date"]')).not.toBeNull();
  });

  it('surfaces a mutation error without losing the calendar', async () => {
    happy();
    vi.mocked(updateWorkout).mockRejectedValue(new ApiError(500, 'failed to update workout'));
    render(<CalendarContainer now={now} />);
    await screen.findByText('Rodazo');
    fireEvent.change(screen.getByLabelText('Estado de Rodazo'), {
      target: { value: 'skipped' },
    });
    expect(await screen.findByRole('alert')).toHaveTextContent('failed to update workout');
    expect(screen.getByText('Rodazo')).toBeInTheDocument();
  });

  it('groups the comparison by view and refreshes after a session change', async () => {
    happy();
    vi.mocked(updateWorkout).mockResolvedValue(rodazo);
    render(<CalendarContainer now={now} />);
    await screen.findByText('Tirada larga');
    expect(loadCalendarComparison).toHaveBeenCalledWith(
      'token',
      '2026-10-26',
      '2026-12-06',
      'week',
      expect.any(AbortSignal),
    );
    fireEvent.click(screen.getByRole('button', { name: 'Semana' }));
    await waitFor(() =>
      expect(loadCalendarComparison).toHaveBeenCalledWith(
        'token',
        '2026-11-23',
        '2026-11-29',
        'day',
        expect.any(AbortSignal),
      ),
    );
    const requests = vi.mocked(loadCalendarComparison).mock.calls.length;
    fireEvent.change(screen.getByLabelText('Estado de Tirada larga'), {
      target: { value: 'skipped' },
    });
    await waitFor(() => expect(loadCalendarComparison).toHaveBeenCalledTimes(requests + 1));
  });

  it('renders an explicit error when the auth token is missing', () => {
    vi.stubEnv('VITE_AUTH_TOKEN', '');
    happy();
    render(<CalendarContainer now={now} />);
    expect(screen.getByRole('alert')).toHaveTextContent('VITE_AUTH_TOKEN');
  });
});
