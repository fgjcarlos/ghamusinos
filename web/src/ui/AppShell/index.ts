// Top-level barrel for AppShell. Consumers (main.tsx) import from
// here. Internal callers inside the folder can still reach the
// individual files.

export { AppShell } from './AppShell';
export type { AppShellProps } from './AppShell';

export { TopBar } from './TopBar';
export type { TopBarProps } from './TopBar';

export { NavLink } from './NavLink';
export type { NavLinkProps } from './NavLink';

export { SyncStatusChip, formatRelativeTime } from './SyncStatusChip';
export type { SyncStatusChipProps } from './SyncStatusChip';
