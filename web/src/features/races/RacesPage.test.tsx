import { afterEach, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { WorkoutForm } from '../workouts/WorkoutForm';
import { RaceDetailRoute, RacesPage } from './RacesPage';

const race = {
  id: 'r1',
  name: 'Sierra Norte',
  race_date: '2030-06-10',
  priority: 'A',
  distance_m: 42000,
  elevation_gain_m: 2200,
  notes: '',
  tracks: [],
  sessions: [],
};
const noProgress = {
  as_of: '2030-05-01',
  window_start: '2030-04-04',
  orientative: true,
  days_remaining: 40,
  weeks_remaining: 5,
  past: false,
  sessions: { total: 0, planned: 0, completed: 0, skipped: 0, modified: 0 },
  activity_count: 0,
  distance_m: 0,
  elevation_gain_m: 0,
  target_distance_m: 42000,
  target_elevation_gain_m: 2200,
  load: [],
};
const track = { id: 'g1', name: 'Cresta' };
const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status });
const empty = () => new Response(null, { status: 204 });
const click = (name: string) => fireEvent.click(screen.getByRole('button', { name }));
const enter = (label: string, value: string) =>
  fireEvent.change(screen.getByLabelText(label), { target: { value } });

function renderRaces(path: string) {
  render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/carreras" element={<RacesPage />} />
        <Route path="/carreras/:id" element={<RaceDetailRoute />} />
      </Routes>
    </MemoryRouter>,
  );
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
  vi.restoreAllMocks();
});

it('creates, edits, attaches and detaches a GPX track, then deletes a race without deleting the track', async () => {
  type Detail = Omit<typeof race, 'tracks'> & { tracks: { id: string; name: string }[] };
  let stored: Detail | null = null;
  const mutations: string[] = [];
  vi.stubGlobal(
    'fetch',
    vi.fn(async (path: string, init?: RequestInit) => {
      const method = init?.method ?? 'GET';
      if (path.startsWith('/api/v1/gpx?'))
        return json({
          data: [{ track: { id: { bytes: track.id }, name: track.name } }],
          has_next: false,
        });
      if (path.startsWith('/api/v1/races?'))
        return json({ data: stored ? [stored] : [], has_next: false });
      if (path.endsWith('/progress')) return json(noProgress);
      if (path.endsWith('/tracks/g1') && stored) {
        stored.tracks = method === 'PUT' ? [track] : [];
        mutations.push(`${method} ${path}`);
        return empty();
      }
      if (method === 'POST' || method === 'PUT') {
        stored = { ...race, ...JSON.parse(String(init?.body)) } as Detail;
        mutations.push(`${method} ${path}`);
        return json(stored, method === 'POST' ? 201 : 200);
      }
      if (method === 'DELETE') {
        mutations.push(`${method} ${path}`);
        stored = null;
        return empty();
      }
      return json(stored);
    }),
  );
  vi.spyOn(window, 'confirm').mockReturnValue(true);
  renderRaces('/carreras');
  expect(await screen.findByText('No hay carreras en esta página.')).toBeInTheDocument();
  click('Nueva carrera');
  enter('Nombre', race.name);
  enter('Fecha', race.race_date);
  enter('Distancia (metros)', '42195');
  enter('Desnivel positivo (metros)', '2200');
  click('Guardar carrera');
  expect(await screen.findByRole('heading', { name: race.name })).toBeInTheDocument();
  click('Editar carrera');
  enter('Nombre', 'Sierra Norte Trail');
  enter('Prioridad', 'B');
  click('Guardar carrera');
  expect(await screen.findByRole('heading', { name: 'Sierra Norte Trail' })).toBeInTheDocument();
  expect(screen.getByText('Prioridad B')).toBeInTheDocument();
  await screen.findByLabelText('Ruta GPX');
  enter('Ruta GPX', track.id);
  click('Vincular ruta');
  expect(await screen.findByRole('link', { name: track.name })).toHaveAttribute(
    'href',
    '/rutas/g1',
  );
  // The only route is now attached, so the picker offers nothing else to link.
  expect(await screen.findByText(/No hay más rutas GPX disponibles/)).toBeInTheDocument();
  expect(screen.getByText(/42,195 km/)).toBeInTheDocument();
  click('Desvincular Cresta');
  expect(await screen.findByText('No hay rutas vinculadas.')).toBeInTheDocument();
  click('Borrar carrera');
  expect(await screen.findByRole('heading', { name: 'Carreras' })).toBeInTheDocument();
  expect(await screen.findByText('No hay carreras en esta página.')).toBeInTheDocument();
  expect(mutations).toEqual([
    'POST /api/v1/races',
    'PUT /api/v1/races/r1',
    'PUT /api/v1/races/r1/tracks/g1',
    'DELETE /api/v1/races/r1/tracks/g1',
    'DELETE /api/v1/races/r1',
  ]);
});

it('shows independent loading, error, retry, and empty states for progress', async () => {
  let finish!: (value: Response) => void;
  const firstResponse = new Promise<Response>((resolve) => {
    finish = resolve;
  });
  let attempts = 0;
  vi.stubGlobal(
    'fetch',
    vi.fn(async (path: string) => {
      if (path.endsWith('/progress')) {
        return ++attempts === 1 ? firstResponse : json(noProgress);
      }
      if (path.startsWith('/api/v1/gpx?')) return json({ data: [], has_next: false });
      return json({
        ...race,
        sessions: [
          { id: 's1', title: 'Tirada larga', scheduled_on: '2030-05-01', status: 'completed' },
        ],
      });
    }),
  );
  renderRaces('/carreras/r1');
  const panel = await screen.findByRole('region', { name: 'Progreso' });
  expect(screen.getByText('Tirada larga')).toBeInTheDocument();
  expect(within(panel).getByRole('status')).toHaveTextContent('Cargando progreso');
  finish(json({}, 503));
  expect(await within(panel).findByRole('alert')).toHaveTextContent(
    'No se pudo cargar el progreso.',
  );
  click('Reintentar progreso');
  expect(
    await within(panel).findByText('No hay sesiones ni actividades para mostrar el progreso.'),
  ).toBeInTheDocument();
  expect(within(panel).getByText(/Métricas orientativas/)).toBeInTheDocument();
  expect(attempts).toBe(2);
});

it('counts one remaining day and week in the singular', async () => {
  vi.stubGlobal(
    'fetch',
    vi.fn(async (path: string) => {
      if (path.endsWith('/progress'))
        return json({ ...noProgress, days_remaining: 1, weeks_remaining: 1 });
      if (path.startsWith('/api/v1/gpx?')) return json({ data: [], has_next: false });
      return json({ ...race, tracks: [], sessions: [] });
    }),
  );
  renderRaces('/carreras/r1');
  const panel = await screen.findByRole('region', { name: 'Progreso' });
  expect(await within(panel).findByText('Falta 1 día (1 semana completa).')).toBeInTheDocument();
});

it('lets a workout choose or clear its race without changing the legacy submission fields', async () => {
  const options = vi.fn(async () => json({ data: [race], has_next: false }));
  vi.stubGlobal('fetch', options);
  vi.stubEnv('VITE_AUTH_TOKEN', 'test-token');
  const save = vi.fn(async () => {});
  const form = (initialId?: string) => (
    <WorkoutForm
      key={initialId ?? 'new'}
      busy={false}
      initial={{
        scheduled_on: '2030-05-01',
        title: 'Cuestas',
        ...(initialId ? { race_id: initialId } : {}),
      }}
      onSave={save}
      onCancel={() => {}}
    />
  );
  const { rerender } = render(form());
  const label = `${race.name} (${race.race_date})`;
  await screen.findByRole('option', { name: label });
  enter('Carrera objetivo', race.id);
  click('Guardar');
  await waitFor(() =>
    expect(save).toHaveBeenLastCalledWith({
      scheduled_on: '2030-05-01',
      title: 'Cuestas',
      notes: '',
      workout_type: 'rodaje',
      target_duration_sec: null,
      target_distance_m: null,
      target_elevation_gain_m: null,
      target_intensity: null,
      race_id: race.id,
    }),
  );
  rerender(form(race.id));
  await screen.findByRole('option', { name: label });
  expect(screen.getByLabelText('Carrera objetivo')).toHaveValue(race.id);
  enter('Carrera objetivo', '');
  click('Guardar');
  await waitFor(() =>
    expect(save).toHaveBeenLastCalledWith(expect.objectContaining({ race_id: null })),
  );
});
