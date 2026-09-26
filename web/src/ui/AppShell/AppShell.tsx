// AppShell. Issue 155.
// Layout wrapper for every private route. Renders TopBar at the top,
// <Outlet /> for the matched child route, and BottomNav at the bottom
// (visible only on narrow viewports, CSS-driven).

import { Outlet } from 'react-router-dom';
import type { ReactNode } from 'react';
import { NavLink } from './NavLink';
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
        <span className={styles.placeholder}>
          <NavLink to="/rendimiento" icon={RENDIMIENTO_ICON}>
            Rendimiento
          </NavLink>
        </span>
      </nav>
    </div>
  );
}
