// TDD contract for ConnectionCard (issue 159).

import { describe, it, expect } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { ConnectionCard } from './ConnectionCard';

describe('ConnectionCard', () => {
  it('renders the disconnected state when not connected', () => {
    render(
      <MemoryRouter>
        <ConnectionCard state={{ connected: false }} />
      </MemoryRouter>,
    );
    expect(screen.getByTestId('strava-connection-disconnected')).toBeInTheDocument();
    expect(
      screen.getByText(/Conecta tu cuenta de Strava para importar/i),
    ).toBeInTheDocument();
  });

  it('renders a connect link pointing at the strava connect endpoint', () => {
    render(
      <MemoryRouter>
        <ConnectionCard state={{ connected: false }} />
      </MemoryRouter>,
    );
    const link = screen.getByTestId('strava-connect-link');
    expect(link).toHaveAttribute('href', '/api/v1/strava/connect');
  });

  it('renders the connected state with athlete_id, scopes, and last_sync', () => {
    render(
      <MemoryRouter>
        <ConnectionCard
          state={{
            connected: true,
            athlete_id: 4242,
            scopes: ['read', 'activity:read'],
            last_sync: '2025-09-15T08:00:00Z',
          }}
        />
      </MemoryRouter>,
    );
    const card = screen.getByTestId('strava-connection-connected');
    expect(within(card).getByText('4242')).toBeInTheDocument();
    expect(within(card).getByText('read, activity:read')).toBeInTheDocument();
  });

  it('does not render scopes row when scopes is empty', () => {
    render(
      <MemoryRouter>
        <ConnectionCard state={{ connected: true, athlete_id: 7, scopes: [] }} />
      </MemoryRouter>,
    );
    const card = screen.getByTestId('strava-connection-connected');
    expect(within(card).queryByText('Scopes')).toBeNull();
  });

  it('does not render last_sync row when last_sync is null', () => {
    render(
      <MemoryRouter>
        <ConnectionCard state={{ connected: true, athlete_id: 7, last_sync: null }} />
      </MemoryRouter>,
    );
    const card = screen.getByTestId('strava-connection-connected');
    expect(within(card).queryByText('Última sincronización')).toBeNull();
  });

  it('shows a busy indicator when busy is true', () => {
    render(
      <MemoryRouter>
        <ConnectionCard
          state={{ connected: true, athlete_id: 7 }}
          busy
        />
      </MemoryRouter>,
    );
    expect(screen.getByText(/Sincronizando/i)).toBeInTheDocument();
  });
});
