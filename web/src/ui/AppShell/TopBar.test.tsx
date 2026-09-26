// TDD contract for TopBar (issue 155).
// Horizontal bar with brand, navigation (3 sections), sync status,
// avatar. Sits at the top of every private route.

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { TopBar } from './TopBar';

describe('TopBar', () => {
  it('renders the brand link that points to /rutas', () => {
    render(
      <MemoryRouter>
        <TopBar lastSync={null} />
      </MemoryRouter>,
    );
    const brand = screen.getByRole('link', { name: /rutas/i });
    expect(brand).toHaveAttribute('href', '/rutas');
  });

  it('renders three navigation links to the documented sections', () => {
    render(
      <MemoryRouter>
        <TopBar lastSync={null} />
      </MemoryRouter>,
    );
    expect(screen.getByRole('link', { name: /Laboratorio/i })).toHaveAttribute('href', '/rutas');
    expect(screen.getByRole('link', { name: /Actividades/i })).toHaveAttribute(
      'href',
      '/actividades',
    );
    expect(screen.getByRole('link', { name: /Rendimiento/i })).toHaveAttribute(
      'href',
      '/rendimiento',
    );
  });

  it('renders Rendimiento with a "Llega en la fase 1.4" placeholder hint', () => {
    render(
      <MemoryRouter>
        <TopBar lastSync={null} />
      </MemoryRouter>,
    );
    const rendimiento = screen.getByRole('link', { name: /Rendimiento/i });
    expect(rendimiento).toHaveAttribute('href', '/rendimiento');
    // The "Llega en la fase 1.4" hint sits next to the link.
    expect(screen.getByText(/Llega en la fase 1\.4/i)).toBeInTheDocument();
  });

  it('renders the SyncStatusChip with the provided date', () => {
    const lastSync = new Date('2026-01-15T11:48:00Z');
    const now = new Date('2026-01-15T12:00:00Z');
    render(
      <MemoryRouter>
        <TopBar lastSync={lastSync} now={now} />
      </MemoryRouter>,
    );
    expect(screen.getByTestId('sync-status')).toHaveTextContent(/Strava · hace 12 min/i);
  });

  it('renders an avatar placeholder', () => {
    render(
      <MemoryRouter>
        <TopBar lastSync={null} avatarInitials="FG" />
      </MemoryRouter>,
    );
    expect(screen.getByTestId('avatar')).toHaveTextContent('FG');
  });
});
