// TDD contract for ErrorBoundary (Fase 1.4, PR4).

import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ErrorBoundary } from './ErrorBoundary';

function Boom(): never {
  throw new Error('boom');
}

describe('ErrorBoundary', () => {
  it('renders the children when nothing fails', () => {
    render(
      <ErrorBoundary>
        <p>contenido</p>
      </ErrorBoundary>,
    );
    expect(screen.getByText('contenido')).toBeInTheDocument();
    expect(screen.queryByTestId('error-boundary-fallback')).toBeNull();
  });

  it('shows the fallback message when a child throws', () => {
    // The boundary logs the error; silence it for this test.
    const spy = vi.spyOn(console, 'error').mockImplementation(() => {});
    render(
      <ErrorBoundary message="No se pudo cargar el panel.">
        <Boom />
      </ErrorBoundary>,
    );
    expect(screen.getByRole('alert')).toHaveTextContent('No se pudo cargar el panel.');
    spy.mockRestore();
  });
});
