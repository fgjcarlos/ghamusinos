// Atomic button. Issue 154. Presentational only: no fetch, no router,
// no useEffect. Variants map to CSS-Module classes via a Record<Variant,
// string> at module scope.

import type { MouseEvent, ReactNode } from 'react';
import styles from './Button.module.css';

export type ButtonVariant = 'primary' | 'secondary' | 'destructive' | 'disabled';

const variantClass: Record<ButtonVariant, string> = {
  primary: styles.primary,
  secondary: styles.secondary,
  destructive: styles.destructive,
  disabled: styles.disabled,
};

export interface ButtonProps {
  variant?: ButtonVariant;
  type?: 'button' | 'submit' | 'reset';
  disabled?: boolean;
  onClick?: (event: MouseEvent<HTMLButtonElement>) => void;
  children: ReactNode;
}

export function Button({
  variant = 'primary',
  type = 'button',
  disabled = false,
  onClick,
  children,
}: ButtonProps) {
  return (
    <button type={type} className={variantClass[variant]} disabled={disabled} onClick={onClick}>
      {children}
    </button>
  );
}
