// TDD contract for AppShell (issue 155).
// AppShell wraps every private route. Renders TopBar at the top and
// the route content via <Outlet />. A BottomNav mirrors the navigation
// for narrow viewports and is hidden at desktop widths (CSS-driven).

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { AppShell } from './AppShell';

describe('AppShell', () => {
  it('renders the TopBar', () => {
    render(
      <MemoryRouter initialEntries={['/rutas']}>
        <Routes>
          <Route element={<AppShell lastSync={null} />}>
            <Route path="/rutas" element={<p>rutas content</p>} />
          </Route>
        </Routes>
      </MemoryRouter>,
    );
    expect(screen.getByRole('banner')).toBeInTheDocument();
  });

  it('renders the Outlet content from the matched child route', () => {
    render(
      <MemoryRouter initialEntries={['/rutas']}>
        <Routes>
          <Route element={<AppShell lastSync={null} />}>
            <Route path="/rutas" element={<p>rutas content</p>} />
            <Route path="/actividades" element={<p>activities content</p>} />
          </Route>
        </Routes>
      </MemoryRouter>,
    );
    expect(screen.getByText('rutas content')).toBeInTheDocument();
  });

  it('renders the BottomNav for narrow viewports', () => {
    render(
      <MemoryRouter initialEntries={['/rutas']}>
        <Routes>
          <Route element={<AppShell lastSync={null} />}>
            <Route path="/rutas" element={<p>rutas content</p>} />
          </Route>
        </Routes>
      </MemoryRouter>,
    );
    // The BottomNav is always in the DOM; CSS controls visibility.
    expect(screen.getByTestId('bottom-nav')).toBeInTheDocument();
  });

  it('renders the child route Outlet (re-renders content on route change)', () => {
    const renderApp = (initial: string) =>
      render(
        <MemoryRouter initialEntries={[initial]}>
          <Routes>
            <Route element={<AppShell lastSync={null} />}>
              <Route path="/rutas" element={<p>rutas content</p>} />
              <Route path="/actividades" element={<p>activities content</p>} />
            </Route>
          </Routes>
        </MemoryRouter>,
      );
    const { unmount } = renderApp('/rutas');
    expect(screen.getByText('rutas content')).toBeInTheDocument();
    unmount();
    // Re-render with a different route. AppShell is the parent layout
    // route and the Outlet shows the matched child.
    renderApp('/actividades');
    expect(screen.getByText('activities content')).toBeInTheDocument();
  });

  it('passes lastSync through to the SyncStatusChip in the TopBar', () => {
    const lastSync = new Date('2026-01-15T11:48:00Z');
    const now = new Date('2026-01-15T12:00:00Z');
    render(
      <MemoryRouter initialEntries={['/rutas']}>
        <Routes>
          <Route element={<AppShell lastSync={lastSync} now={now} avatarInitials="FG" />}>
            <Route path="/rutas" element={<p>rutas content</p>} />
          </Route>
        </Routes>
      </MemoryRouter>,
    );
    expect(screen.getByTestId('sync-status')).toHaveTextContent(/Strava · hace 12 min/i);
    expect(screen.getByTestId('avatar')).toHaveTextContent('FG');
  });
});
