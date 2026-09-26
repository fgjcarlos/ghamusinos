// TopBar. Issue 155.
// Horizontal header that sits above the AppShell outlet.
// Brand on the left, three sections (with the third as an honest
// placeholder), sync indicator, avatar.

import { Link } from 'react-router-dom';
import type { ReactNode } from 'react';
import { NavLink } from './NavLink';
import { SyncStatusChip } from './SyncStatusChip';
import styles from './TopBar.module.css';

const BRAND_ICON: ReactNode = (
  <svg viewBox="0 0 24 24" aria-hidden="true">
    <path d="M2 19 L8 7 L12 14 L16 4 L22 19" />
  </svg>
);

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

export interface TopBarProps {
  lastSync: Date | null;
  /** Reference clock for SyncStatusChip's relative-time formatter. */
  now?: Date;
  /** Initials shown inside the avatar circle. Empty string renders the dot. */
  avatarInitials?: string;
}

export function TopBar({ lastSync, now, avatarInitials = '' }: TopBarProps) {
  return (
    <header className={styles.bar} role="banner">
      <Link to="/rutas" className={styles.brand} aria-label="Ir a rutas">
        <span className={styles.brandIcon}>{BRAND_ICON}</span>
        <span className={styles.brandName}>Ghamusinos</span>
      </Link>

      <nav className={styles.nav} aria-label="Principal">
        <NavLink to="/rutas" icon={LAB_ICON}>
          Laboratorio
        </NavLink>
        <NavLink to="/actividades" icon={ACTIVITIES_ICON}>
          Actividades
        </NavLink>
        <span className={styles.placeholder} data-state="placeholder">
          <NavLink to="/rendimiento" icon={RENDIMIENTO_ICON}>
            Rendimiento
          </NavLink>
          <span className={styles.placeholderHint}>Llega en la fase 1.4</span>
        </span>
      </nav>

      <div className={styles.right}>
        <SyncStatusChip lastSync={lastSync} {...(now !== undefined ? { now } : {})} />
        <span className={styles.avatar} data-testid="avatar" aria-label="Tu perfil">
          {avatarInitials || ''}
        </span>
      </div>
    </header>
  );
}
