// TDD contract for the route tree (Fase 1.4, DW-001).
// The dashboard lives at /dashboard and the legacy /rendimiento alias
// redirects to it. The dashboard route is lazy loaded (ECharts lives in
// its own chunk), so the assertions await the lazy chunk to resolve.

import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { AppRoutes } from './AppRoutes';

vi.mock('./lib/api/workouts', () => ({
  listWorkouts: vi
    .fn()
    .mockResolvedValue({ data: [], total: 0, limit: 20, offset: 0, has_next: false }),
  createWorkout: vi.fn(),
  updateWorkout: vi.fn(),
  deleteWorkout: vi.fn(),
}));

describe('AppRoutes', () => {
  // The dashboard route is lazy loaded, so the first render has to
  // transform the whole dashboard module graph (ECharts included).
  // Under a parallel test run that can take longer than the default
  // 1s wait: the assertions below allow 5s.
  const LAZY_TIMEOUT = { timeout: 5000 };

  it('mounts planned workouts at the SPA route', async () => {
    render(
      <MemoryRouter initialEntries={['/entrenos']}>
        <AppRoutes />
      </MemoryRouter>,
    );
    expect(await screen.findByText(/Próximas sesiones:/)).toBeInTheDocument();
    expect(document.querySelectorAll('main')).toHaveLength(1);
    expect(screen.getByRole('heading', { name: 'Entrenos' })).toBeInTheDocument();
    expect(
      screen.getByTestId('bottom-nav').querySelector('a[href="/entrenos"]'),
    ).toBeInTheDocument();
  });

  it('mounts the dashboard page on /dashboard', async () => {
    render(
      <MemoryRouter initialEntries={['/dashboard']}>
        <AppRoutes />
      </MemoryRouter>,
    );
    expect(
      await screen.findByTestId('period-selector', undefined, LAZY_TIMEOUT),
    ).toBeInTheDocument();
  });

  it('redirects the legacy /rendimiento route to /dashboard', async () => {
    render(
      <MemoryRouter initialEntries={['/rendimiento']}>
        <AppRoutes />
      </MemoryRouter>,
    );
    expect(
      await screen.findByTestId('period-selector', undefined, LAZY_TIMEOUT),
    ).toBeInTheDocument();
  });

  it('keeps the dashboard inside the AppShell navigation', async () => {
    render(
      <MemoryRouter initialEntries={['/dashboard']}>
        <AppRoutes />
      </MemoryRouter>,
    );
    const nav = screen.getByTestId('bottom-nav');
    expect(nav.querySelector('a[href="/dashboard"]')).not.toBeNull();
  });
});
