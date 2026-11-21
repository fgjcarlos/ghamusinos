import { fireEvent, render, screen, within } from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import { loadCalendarComparison } from '../../lib/api/calendar';
import { createDeferred } from '../../test/deferred';
import { useEChart } from '../dashboard/useEChart';
import { CalendarComparison } from './CalendarComparison';

vi.mock('../../lib/api/calendar', () => ({ loadCalendarComparison: vi.fn() }));
vi.mock('../dashboard/useEChart', () => ({ useEChart: vi.fn(() => ({ current: null })) }));

const props = { token: 'token', from: '2026-11-23', to: '2026-11-29', group: 'day' } as const;
const period = {
  period_start: '2026-11-23',
  planned: { duration_seconds: 3600, distance_m: 5000, elevation_gain_m: 300 },
  actual: { duration_seconds: 3120, distance_m: 6200, elevation_gain_m: 410 },
  compliance: { total: 3, completed: 1, skipped: 1, modified: 1 },
};

beforeEach(() => vi.clearAllMocks());

it('shows loading and then an explicit empty state', async () => {
  const response = createDeferred<{ periods: (typeof period)[] }>();
  vi.mocked(loadCalendarComparison).mockReturnValueOnce(response.promise);
  render(<CalendarComparison {...props} />);
  expect(screen.getByRole('status')).toHaveTextContent('Cargando comparación');
  response.resolve({ periods: [] });
  expect(
    await screen.findByText('No hay carga planificada ni realizada en este periodo.'),
  ).toBeVisible();
  expect(screen.queryByText('Cargando comparación…')).not.toBeInTheDocument();
});

it('shows failed requests and retries the same range', async () => {
  vi.mocked(loadCalendarComparison)
    .mockRejectedValueOnce(new Error('offline'))
    .mockResolvedValueOnce({ periods: [period] });
  render(<CalendarComparison {...props} />);
  expect(await screen.findByRole('alert')).toHaveTextContent('No se pudo cargar la comparación.');
  fireEvent.click(screen.getByRole('button', { name: 'Reintentar comparación' }));
  expect(await screen.findByLabelText('Métrica')).toBeInTheDocument();
  expect(vi.mocked(loadCalendarComparison)).toHaveBeenNthCalledWith(
    2,
    'token',
    props.from,
    props.to,
    'day',
    expect.any(AbortSignal),
  );
  expect(screen.queryByRole('alert')).not.toBeInTheDocument();
});

it('compares the three metrics in grouped bars and exposes period values without canvas', async () => {
  vi.mocked(loadCalendarComparison).mockResolvedValueOnce({ periods: [period] });
  render(<CalendarComparison {...props} />);
  const metric = await screen.findByLabelText('Métrica');
  const expectBars = (planned: number, actual: number) =>
    expect(useEChart).toHaveBeenLastCalledWith(
      expect.objectContaining({
        xAxis: expect.objectContaining({ data: [period.period_start] }),
        series: [
          expect.objectContaining({ type: 'bar', data: [planned] }),
          expect.objectContaining({ type: 'bar', data: [actual] }),
        ],
      }),
    );
  expectBars(60, 52);
  fireEvent.change(metric, { target: { value: 'distance_m' } });
  expectBars(5, 6.2);
  fireEvent.change(metric, { target: { value: 'elevation_gain_m' } });
  expectBars(300, 410);
  fireEvent.click(screen.getByText('Ver datos y cumplimiento'));
  const row = screen.getByRole('row', { name: /2026-11-23/ });
  expect(within(row).getByText('410')).toBeVisible();
  expect(within(row).getByText('3')).toBeVisible();
});
