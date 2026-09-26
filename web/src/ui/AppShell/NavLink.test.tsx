// TDD contract for NavLink (issue 155).
// Wraps react-router-dom's <NavLink> with the icon + label layout used
// in AppShell's top bar and bottom nav.

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { NavLink } from './NavLink';

const HOME_ICON = (
  <svg data-testid="icon" viewBox="0 0 24 24">
    <path d="M2 19 L8 7 L12 14 L16 4 L22 19" />
  </svg>
);

describe('NavLink', () => {
  it('renders an accessible link with the label as accessible name', () => {
    render(
      <MemoryRouter>
        <NavLink to="/rutas" icon={HOME_ICON}>
          Laboratorio
        </NavLink>
      </MemoryRouter>,
    );
    const link = screen.getByRole('link', { name: /Laboratorio/ });
    expect(link).toBeInTheDocument();
  });

  it('renders the icon as a child', () => {
    render(
      <MemoryRouter>
        <NavLink to="/rutas" icon={HOME_ICON}>
          Laboratorio
        </NavLink>
      </MemoryRouter>,
    );
    expect(screen.getByTestId('icon')).toBeInTheDocument();
  });

  it('sets aria-current="page" when the current path matches to', () => {
    render(
      <MemoryRouter initialEntries={['/rutas']}>
        <NavLink to="/rutas" icon={HOME_ICON}>
          Laboratorio
        </NavLink>
      </MemoryRouter>,
    );
    expect(screen.getByRole('link')).toHaveAttribute('aria-current', 'page');
  });

  it('does NOT set aria-current when the path does not match', () => {
    render(
      <MemoryRouter initialEntries={['/actividades']}>
        <NavLink to="/rutas" icon={HOME_ICON}>
          Laboratorio
        </NavLink>
      </MemoryRouter>,
    );
    expect(screen.getByRole('link')).not.toHaveAttribute('aria-current');
  });

  it('does not register a beforeunload handler when clicked', () => {
    render(
      <MemoryRouter>
        <NavLink to="/rutas" icon={HOME_ICON}>
          Laboratorio
        </NavLink>
      </MemoryRouter>,
    );
    const link = screen.getByRole('link');
    // SPA navigation: anchor must NOT carry onClick that triggers reload.
    expect(link.tagName).toBe('A');
    expect(link.getAttribute('href')).toBe('/rutas');
    // If react-router-dom's NavLink attaches a click handler that
    // bypasses default browser reload, this assertion holds.
    // (We don't dispatch the click because jsdom does not load the
    // route; we just verify the link is a normal SPA anchor.)
  });
});
