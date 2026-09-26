// Top-level barrel for the design-system primitives in src/ui/.
// Consumers import from here, never from the per-component folders.
// Each export stays pure (presentation-only) and free of domain
// knowledge — they do not import from features/, routes/, or lib/api/.

export { Button } from './Button';
export type { ButtonProps, ButtonVariant } from './Button';

export { Chip } from './Chip';
export type { ChipProps, ChipTone } from './Chip';

export { DifficultyBadge } from './DifficultyBadge';
export type { DifficultyBadgeProps, DifficultyLevel } from './DifficultyBadge';

export { SeverityPill } from './SeverityPill';
export type { SeverityPillProps, SeverityLevel } from './SeverityPill';

export { Field } from './Field';
export type { FieldProps } from './Field';

export { MetricTile } from './MetricTile';
export type { MetricTileProps } from './MetricTile';

export { EmptyState } from './EmptyState';
export type { EmptyStateProps } from './EmptyState';
