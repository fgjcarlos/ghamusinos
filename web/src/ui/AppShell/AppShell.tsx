// AppShell. Issue 155.
// Layout wrapper for every private route. Renders TopBar at the top,
// <Outlet /> for the matched child route, and BottomNav at the bottom
// (visible only on narrow viewports, CSS-driven).

import { Outlet } from 'react-router-dom';
import type { ReactNode } from 'react';
import { NavLink } from './NavLink';
import { RaceFlag } from './RaceFlag';
import { TopBar } from './TopBar';
import styles from './AppShell.module.css';

const LAB_ICON: ReactNode = (
  <svg viewBox="0 0 24 24" aria-hidden="true">
    <path d="M3 18 L9 9 L13 14 L17 6 L21 18" />
  </svg>
);

const ACTIVITIES_ICON: ReactNode = (
  <svg viewBox="0 0 24 24" aria-hidden="true">
    <line x1="4" y1="6" x2="20" y2="6" />
    <line x1="4" y1="12" x2="20" y2="12" />
    <line x1="4" y1="18" x2="20" y2="18" />
  </svg>
);

const PLANNED_ICON: ReactNode = (
  <svg viewBox="0 0 24 24" aria-hidden="true">
    <rect x="3" y="5" width="18" height="16" rx="2" />
    <line x1="7" y1="3" x2="7" y2="7" />
    <line x1="17" y1="3" x2="17" y2="7" />
    <line x1="3" y1="10" x2="21" y2="10" />
  </svg>
);

const CALENDAR_ICON: ReactNode = (
  <svg viewBox="0 0 24 24" aria-hidden="true">
    <rect x="3" y="5" width="18" height="16" rx="2" />
    <line x1="3" y1="10" x2="21" y2="10" />
    <rect x="6" y="13" width="4" height="4" rx="1" />
    <rect x="13" y="13" width="4" height="4" rx="1" />
  </svg>
);

const RENDIMIENTO_ICON: ReactNode = (
  <svg viewBox="0 0 24 24" aria-hidden="true">
    <path d="M3 12 H7 L10 6 L14 18 L17 12 H21" />
  </svg>
);

export interface AppShellProps {
  lastSync: Date | null;
  now?: Date;
  avatarInitials?: string;
}

export function AppShell({ lastSync, now, avatarInitials }: AppShellProps) {
  return (
    <div className={styles.shell}>
      <TopBar
        lastSync={lastSync}
        {...(now !== undefined ? { now } : {})}
        {...(avatarInitials !== undefined ? { avatarInitials } : {})}
      />
      <main className={styles.main}>
        <Outlet />
      </main>
      <nav className={styles.bottomNav} data-testid="bottom-nav" aria-label="Principal (móvil)">
        <NavLink to="/rutas" icon={LAB_ICON}>
          Laboratorio
        </NavLink>
        <NavLink to="/actividades" icon={ACTIVITIES_ICON}>
          Actividades
        </NavLink>
        <NavLink to="/entrenos" icon={PLANNED_ICON}>
          Entrenos
        </NavLink>
        <NavLink to="/carreras" icon={<RaceFlag />} end={false}>
          Carreras
        </NavLink>
        <NavLink to="/calendario" icon={CALENDAR_ICON}>
          Calendario
        </NavLink>
        <NavLink to="/dashboard" icon={RENDIMIENTO_ICON}>
          Rendimiento
        </NavLink>
        <NavLink to="/reportes" icon={RENDIMIENTO_ICON}>
          Reportes
        </NavLink>
      </nav>
    </div>
  );
}
