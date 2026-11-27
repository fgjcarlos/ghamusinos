// TDD contract for the /reportes screen (fase 2.4, #320 · S1, S2, S5).
// Fetch is stubbed exactly like RacesPage.test.tsx: the API clients own
// the bearer token, so the tests assert the URL and the body the screen
// sends, never the transport.

import { afterEach, expect, it, vi } from 'vitest';
import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { AppRoutes } from '../../AppRoutes';
import { ReportsPage } from './ReportsPage';

const page = <T,>(data: T[], hasNext = false) => ({
  data,
  total: data.length,
  limit: 20,
  offset: 0,
  has_next: hasNext,
});
const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status });
const click = (name: string) => fireEvent.click(screen.getByRole('button', { name }));
const enter = (label: string, value: string) =>
  fireEvent.change(screen.getByLabelText(label), { target: { value } });
const flush = async () => {
  await act(async () => {
    await Promise.resolve();
    await Promise.resolve();
  });
};
const renderPage = () =>
  render(
    <MemoryRouter>
      <ReportsPage />
    </MemoryRouter>,
  );

const weekItem = {
  id: 'rep1',
  kind: 'week',
  period_start: '2030-03-04',
  period_end: '2030-03-10',
  block_id: null,
  generated_at: '2030-03-11T06:00:00Z',
  summary: {
    tss: 321.4,
    activities: 3,
    distance_m: 42300,
    elevation_gain_m: 1200,
    avg_ef: null,
    avg_gap_sec_per_km: null,
    adherence: 0.67,
  },
};

const payload = {
  schema_version: 1,
  kind: 'week',
  period_start: '2030-03-04',
  period_end: '2030-03-10',
  days: 7,
  block: null,
  race: null,
  load: {
    tss: 321.4,
    daily: [
      { date: '2030-03-04', tss: 40, activity_count: 1 },
      { date: '2030-03-05', tss: 0, activity_count: 0 },
    ],
    ctl_start: 40,
    atl_start: 45,
    tsb_start: -5,
    ctl_end: 44,
    atl_end: 50,
    tsb_end: -6,
  },
  volume: { activities: 3, distance_m: 42300, elevation_gain_m: 1200, moving_seconds: 14000 },
  performance: { avg_ef: null, ef_samples: 0, avg_gap_sec_per_km: null, gap_samples: 0 },
  plan: { planned: 3, completed: 2, skipped: 1, modified: 0, adherence: 0.67 },
  previous: {
    period_start: '2030-02-25',
    period_end: '2030-03-03',
    activities: 2,
    tss: 280,
    distance_m: 30000,
    elevation_gain_m: 800,
    avg_ef: null,
    avg_gap_sec_per_km: null,
  },
};

const block = {
  id: 'b1',
  name: 'Base',
  start_date: '2030-01-01',
  end_date: '2030-02-01',
  race_id: null,
  notes: '',
  created_at: '2030-01-01T00:00:00Z',
  updated_at: '2030-01-01T00:00:00Z',
};

const race = {
  id: 'r1',
  name: 'Sierra Norte',
  race_date: '2030-06-10',
  priority: 'A',
  distance_m: 42000,
  elevation_gain_m: 2200,
  notes: '',
  created_at: '2030-01-01T00:00:00Z',
  updated_at: '2030-01-01T00:00:00Z',
};

afterEach(() => {
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
  vi.restoreAllMocks();
});

it('shows the loading state and then the empty state of the report list', async () => {
  let finish!: (value: Response) => void;
  vi.stubEnv('VITE_AUTH_TOKEN', 'test-token');
  vi.stubGlobal(
    'fetch',
    vi.fn(async (path: string) => {
      if (path.startsWith('/api/v1/reports?'))
        return new Promise<Response>((resolve) => {
          finish = resolve;
        });
      return json(page([]));
    }),
  );
  renderPage();
  expect(screen.getByText('Cargando reportes…')).toBeInTheDocument();
  finish(json(page([])));
  expect(await screen.findByText('Todavía no hay reportes de este periodo.')).toBeInTheDocument();
});

it('shows the error state and retries the list', async () => {
  let calls = 0;
  vi.stubGlobal(
    'fetch',
    vi.fn(async (path: string) => {
      if (path.startsWith('/api/v1/reports?'))
        return ++calls === 1 ? json({ detail: 'boom' }, 500) : json(page([weekItem]));
      return json(page([]));
    }),
  );
  renderPage();
  expect(await screen.findByText('No se pudieron cargar los reportes.')).toBeInTheDocument();
  click('Reintentar');
  expect(
    await screen.findByRole('button', { name: '2030-03-04 – 2030-03-10' }),
  ).toBeInTheDocument();
});

it('refetches the list with the chosen kind and shows the pending generation', async () => {
  const urls: string[] = [];
  const bodies: unknown[] = [];
  vi.stubGlobal(
    'fetch',
    vi.fn(async (path: string, init?: RequestInit) => {
      if (path === '/api/v1/reports/generate') {
        bodies.push(JSON.parse(String(init?.body)));
        return json(
          { status: 'queued', kind: 'month', period_start: '2030-03-01', period_end: '2030-03-31' },
          202,
        );
      }
      if (path.startsWith('/api/v1/reports?')) {
        urls.push(path);
        return json(page([]));
      }
      return json(page([]));
    }),
  );
  renderPage();
  await screen.findByText('Todavía no hay reportes de este periodo.');
  expect(urls[0]).toContain('kind=week');
  enter('Tipo de periodo', 'month');
  await waitFor(() => expect(urls.some((url) => url.includes('kind=month'))).toBe(true));
  enter('Día del periodo', '2030-03-04');
  click('Generar reporte');
  expect(
    await screen.findByText(/Generando el reporte 2030-03-01 – 2030-03-31/),
  ).toBeInTheDocument();
  expect(bodies).toEqual([{ kind: 'month', date: '2030-03-04' }]);
});

it('renders the detail numbers and no data for the null EF and GAP', async () => {
  vi.stubGlobal(
    'fetch',
    vi.fn(async (path: string) => {
      if (path === '/api/v1/reports/rep1')
        return json({ id: 'rep1', generated_at: '2030-03-11T06:00:00Z', payload });
      if (path.startsWith('/api/v1/reports?')) return json(page([weekItem]));
      return json(page([]));
    }),
  );
  renderPage();
  const list = screen.getByRole('region', { name: 'Reportes de semana' });
  await within(list).findByRole('button', { name: '2030-03-04 – 2030-03-10' });
  expect(within(list).getByText(/321,4/)).toBeInTheDocument();
  click('2030-03-04 – 2030-03-10');
  const detail = await screen.findByRole('region', { name: 'Detalle del reporte' });
  await within(detail).findByText('321,4');
  expect(within(detail).getByText('42,3 km')).toBeInTheDocument();
  expect(within(detail).getByText('1200 m')).toBeInTheDocument();
  expect(within(detail).getAllByText('Sin datos')).toHaveLength(2);
  expect(within(detail).getByText('67 %')).toBeInTheDocument();
  expect(within(detail).getByTestId('daily-load-chart')).toBeInTheDocument();
  expect(within(detail).getByText(/Métricas orientativas/)).toBeInTheDocument();
  expect(within(detail).getByText('2030-02-25 – 2030-03-03')).toBeInTheDocument();
});

it('selects the report once the queued generation shows up in the list', async () => {
  let generated = false;
  vi.stubGlobal(
    'fetch',
    vi.fn(async (path: string) => {
      if (path === '/api/v1/reports/generate') {
        generated = true;
        return json(
          { status: 'queued', kind: 'week', period_start: '2030-03-04', period_end: '2030-03-10' },
          202,
        );
      }
      if (path === '/api/v1/reports/rep1')
        return json({ id: 'rep1', generated_at: '2030-03-11T06:00:00Z', payload });
      if (path.startsWith('/api/v1/reports?')) return json(page(generated ? [weekItem] : []));
      return json(page([]));
    }),
  );
  renderPage();
  await screen.findByText('Todavía no hay reportes de este periodo.');
  click('Generar reporte');
  const detail = await screen.findByRole('region', { name: 'Detalle del reporte' });
  await within(detail).findByText('321,4');
  expect(screen.queryByText(/Generando el reporte/)).not.toBeInTheDocument();
});

it('keeps the block form values and shows the overlap message on 409', async () => {
  vi.stubGlobal(
    'fetch',
    vi.fn(async (path: string, init?: RequestInit) => {
      if (init?.method === 'POST' && path === '/api/v1/blocks')
        return json({ detail: 'training block overlaps an existing block' }, 409);
      if (path.startsWith('/api/v1/races?')) return json(page([race]));
      return json(page([]));
    }),
  );
  renderPage();
  await screen.findByText('Todavía no hay bloques.');
  click('Nuevo bloque');
  await screen.findByRole('option', { name: 'Sierra Norte' });
  enter('Nombre', 'Base');
  enter('Inicio', '2030-01-01');
  enter('Fin', '2030-02-01');
  enter('Carrera objetivo', 'r1');
  click('Guardar bloque');
  expect(
    await screen.findByText('El bloque se solapa con otro bloque existente.'),
  ).toBeInTheDocument();
  expect(screen.getByLabelText('Nombre')).toHaveValue('Base');
  expect(screen.getByLabelText('Inicio')).toHaveValue('2030-01-01');
  expect(screen.getByLabelText('Fin')).toHaveValue('2030-02-01');
  expect(screen.getByLabelText('Carrera objetivo')).toHaveValue('r1');
});

it('mounts the reports screen at /reportes inside the shell navigation', async () => {
  vi.stubGlobal(
    'fetch',
    vi.fn(async () => json(page([]))),
  );
  render(
    <MemoryRouter initialEntries={['/reportes']}>
      <AppRoutes />
    </MemoryRouter>,
  );
  expect(
    await screen.findByRole('heading', { name: 'Reportes' }, { timeout: 5000 }),
  ).toBeInTheDocument();
  expect(screen.getByTestId('bottom-nav').querySelector('a[href="/reportes"]')).toHaveTextContent(
    'Reportes',
  );
});

it('edits a block and refreshes the list', async () => {
  let blocks = [block];
  const puts: { path: string; body: unknown }[] = [];
  vi.stubGlobal(
    'fetch',
    vi.fn(async (path: string, init?: RequestInit) => {
      if (init?.method === 'PUT' && path === '/api/v1/blocks/b1') {
        puts.push({ path, body: JSON.parse(String(init?.body)) });
        blocks = [{ ...blocks[0]!, name: 'Base 2', notes: 'Carga' }];
        return json(blocks[0]);
      }
      if (path.startsWith('/api/v1/blocks?')) return json(page(blocks));
      if (path.startsWith('/api/v1/races?')) return json(page([race]));
      return json(page([]));
    }),
  );
  renderPage();
  await screen.findByRole('button', { name: 'Editar bloque Base' });
  click('Editar bloque Base');
  expect(screen.getByLabelText('Nombre')).toHaveValue('Base');
  expect(screen.getByLabelText('Inicio')).toHaveValue('2030-01-01');
  expect(screen.getByLabelText('Fin')).toHaveValue('2030-02-01');
  enter('Nombre', 'Base 2');
  enter('Notas', 'Carga');
  click('Guardar bloque');
  expect(await screen.findByRole('button', { name: 'Editar bloque Base 2' })).toBeInTheDocument();
  expect(puts).toEqual([
    {
      path: '/api/v1/blocks/b1',
      body: {
        name: 'Base 2',
        start_date: '2030-01-01',
        end_date: '2030-02-01',
        race_id: null,
        notes: 'Carga',
      },
    },
  ]);
});

it('deletes a block and refreshes the list', async () => {
  let blocks = [block];
  const deletes: string[] = [];
  vi.stubGlobal(
    'fetch',
    vi.fn(async (path: string, init?: RequestInit) => {
      if (init?.method === 'DELETE') {
        deletes.push(path);
        blocks = [];
        return new Response(null, { status: 204 });
      }
      if (path.startsWith('/api/v1/blocks?')) return json(page(blocks));
      if (path.startsWith('/api/v1/races?')) return json(page([race]));
      return json(page([]));
    }),
  );
  vi.spyOn(window, 'confirm').mockReturnValue(true);
  renderPage();
  await screen.findByRole('button', { name: 'Borrar bloque Base' });
  click('Borrar bloque Base');
  expect(await screen.findByText('Todavía no hay bloques.')).toBeInTheDocument();
  expect(deletes).toEqual(['/api/v1/blocks/b1']);
});

it('keeps the block and shows an error when deleting fails', async () => {
  vi.stubGlobal(
    'fetch',
    vi.fn(async (path: string, init?: RequestInit) => {
      if (init?.method === 'DELETE') return json({ detail: 'boom' }, 500);
      if (path.startsWith('/api/v1/blocks?')) return json(page([block]));
      if (path.startsWith('/api/v1/races?')) return json(page([race]));
      return json(page([]));
    }),
  );
  vi.spyOn(window, 'confirm').mockReturnValue(true);
  renderPage();
  await screen.findByRole('button', { name: 'Borrar bloque Base' });
  click('Borrar bloque Base');
  expect(await screen.findByText('No se pudo borrar el bloque.')).toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'Borrar bloque Base' })).toBeInTheDocument();
});

it('shows the validation message and keeps the values on 422', async () => {
  vi.stubGlobal(
    'fetch',
    vi.fn(async (path: string, init?: RequestInit) => {
      if (init?.method === 'POST' && path === '/api/v1/blocks')
        return json({ detail: 'invalid' }, 422);
      if (path.startsWith('/api/v1/races?')) return json(page([race]));
      return json(page([]));
    }),
  );
  renderPage();
  await screen.findByText('Todavía no hay bloques.');
  click('Nuevo bloque');
  enter('Nombre', 'Base');
  enter('Inicio', '2030-02-01');
  enter('Fin', '2030-01-01');
  click('Guardar bloque');
  expect(
    await screen.findByText('Revisa los datos: el nombre y el rango de fechas no son válidos.'),
  ).toBeInTheDocument();
  expect(screen.getByLabelText('Inicio')).toHaveValue('2030-02-01');
  expect(screen.getByLabelText('Fin')).toHaveValue('2030-01-01');
});

it('stops polling after a bounded number of refreshes and lets the user check again', async () => {
  vi.useFakeTimers();
  let listCalls = 0;
  vi.stubGlobal(
    'fetch',
    vi.fn(async (path: string) => {
      if (path === '/api/v1/reports/generate')
        return json(
          { status: 'queued', kind: 'week', period_start: '2030-03-04', period_end: '2030-03-10' },
          202,
        );
      if (path.startsWith('/api/v1/reports?')) {
        listCalls += 1;
        return json(page([]));
      }
      return json(page([]));
    }),
  );
  renderPage();
  await flush();
  expect(screen.getByText('Todavía no hay reportes de este periodo.')).toBeInTheDocument();
  expect(listCalls).toBe(1);
  click('Generar reporte');
  await flush();
  expect(listCalls).toBe(2);
  expect(screen.getByText(/Generando el reporte/)).toBeInTheDocument();
  // 2 s per attempt, at most MAX_POLLS attempts: advancing far past the
  // bound must not keep firing requests.
  for (let poll = 0; poll < 10; poll += 1) {
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2000);
    });
    await flush();
  }
  expect(listCalls).toBe(7);
  expect(screen.getByText(/todavía no está listo/)).toBeInTheDocument();
  await act(async () => {
    await vi.advanceTimersByTimeAsync(60_000);
  });
  await flush();
  expect(listCalls).toBe(7);
  click('Volver a comprobar');
  await flush();
  expect(listCalls).toBe(8);
});
