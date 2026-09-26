// NavLink wrapper. Issue 155.
// Builds on react-router-dom's <NavLink> so that the active state comes
// from the router's match (no manual pathname comparison).

import type { ReactNode } from 'react';
import { NavLink as RouterNavLink } from 'react-router-dom';
import styles from './NavLink.module.css';

export interface NavLinkProps {
  to: string;
  icon: ReactNode;
  children: ReactNode;
  /** Override end-match if needed (default true). */
  end?: boolean;
}

export function NavLink({ to, icon, children, end = true }: NavLinkProps) {
  return (
    <RouterNavLink
      to={to}
      end={end}
      className={({ isActive }) => (isActive ? styles.active : styles.link)}
    >
      <span className={styles.icon} aria-hidden="true">
        {icon}
      </span>
      <span className={styles.label}>{children}</span>
    </RouterNavLink>
  );
}
