// TDD contract for the route tree (Fase 1.4, DW-001).
// The dashboard lives at /dashboard and the legacy /rendimiento alias
// redirects to it. The dashboard route is lazy loaded (ECharts lives in
// its own chunk), so the assertions await the lazy chunk to resolve.

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { AppRoutes } from './AppRoutes';

describe('AppRoutes', () => {
  // The dashboard route is lazy loaded, so the first render has to
  // transform the whole dashboard module graph (ECharts included).
  // Under a parallel test run that can take longer than the default
  // 1s wait: the assertions below allow 5s.
  const LAZY_TIMEOUT = { timeout: 5000 };

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
