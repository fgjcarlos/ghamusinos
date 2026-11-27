// Pins the default period day to the user's local calendar. The initial
// `date` of the week/month selector used `toISOString()`, which is UTC:
// west of UTC it names the previous day. The test forces a zone where the
// two differ (2030-03-04T04:30Z is 2030-03-03 23:30 in New York).

import { afterAll, afterEach, beforeAll, expect, it, vi } from 'vitest';
import { act, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { ReportsPage } from './ReportsPage';
import { localDay } from './report-format';

const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status });
const emptyPage = { data: [], total: 0, limit: 20, offset: 0, has_next: false };
/** 2030-03-03 23:30 in New York, already 2030-03-04 in UTC. */
const instant = new Date('2030-03-04T04:30:00Z');

beforeAll(() => {
  // Forced here, not read from the machine: the test must not depend on
  // the CI timezone.
  vi.stubEnv('TZ', 'America/New_York');
});

afterAll(() => {
  vi.unstubAllEnvs();
});

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

it('defaults the period day to the local civil day, not the UTC one', async () => {
  vi.useFakeTimers({ now: instant });
  vi.stubGlobal(
    'fetch',
    vi.fn(async () => json(emptyPage)),
  );
  render(
    <MemoryRouter>
      <ReportsPage />
    </MemoryRouter>,
  );
  await act(async () => {});
  expect(screen.getByLabelText('Día del periodo')).toHaveValue('2030-03-03');
});

it('localDay formats the local calendar day', () => {
  // The forced zone is in effect: the same instant is the 3rd there.
  expect(instant.getDate()).toBe(3);
  expect(localDay(instant)).toBe('2030-03-03');
  // The formatter the default replaced: UTC, so the previous day here.
  expect(instant.toISOString().slice(0, 10)).toBe('2030-03-04');
});
