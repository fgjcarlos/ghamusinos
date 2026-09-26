// TDD contract for EmptyState (issue 154).

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { EmptyState } from './EmptyState';

describe('EmptyState', () => {
  it('renders the message as visible text', () => {
    render(<EmptyState message="Sin actividades todavía" />);
    expect(screen.getByText('Sin actividades todavía')).toBeInTheDocument();
  });

  it('renders the action when provided', () => {
    render(<EmptyState message="Sin actividades" action={<button>Sincronizar</button>} />);
    expect(screen.getByRole('button', { name: 'Sincronizar' })).toBeInTheDocument();
  });

  it('does not render any action when omitted', () => {
    render(<EmptyState message="Vacío" />);
    expect(screen.queryByRole('button')).toBeNull();
  });

  it('renders the icon when provided', () => {
    render(<EmptyState icon={<svg data-testid="my-icon" />} message="Hola" />);
    expect(screen.getByTestId('my-icon')).toBeInTheDocument();
  });

  it('does not render the icon when omitted', () => {
    render(<EmptyState message="Hola" />);
    expect(screen.queryByTestId('my-icon')).toBeNull();
  });
});
