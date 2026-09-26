// TDD contract for Field (issue 154).

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { Field } from './Field';

describe('Field', () => {
  it('renders the label as visible text', () => {
    render(<Field label="Email">{<input />}</Field>);
    expect(screen.getByText('Email')).toBeInTheDocument();
  });

  it('renders the unit inline next to the label when provided', () => {
    render(
      <Field label="Distancia" unit="km">
        {<input />}
      </Field>,
    );
    // data-testid gives a stable selector that doesn't depend on
    // the (hashed) CSS-Module class names or fragment-level text
    // boundaries. The unit is rendered inside parens.
    expect(screen.getByTestId('field-label')).toHaveTextContent('Distancia');
    expect(screen.getByTestId('field-unit')).toHaveTextContent('(km)');
  });

  it('does not render any unit suffix when unit is omitted', () => {
    render(<Field label="Notas">{<input />}</Field>);
    // Without a unit, the label has no extra suffix line.
    const labelEl = screen.getByText('Notas');
    expect(labelEl.textContent).toBe('Notas');
  });

  it('renders the error message inside role="alert" when error is set', () => {
    render(
      <Field label="Email" error="Requerido">
        {<input />}
      </Field>,
    );
    const alert = screen.getByRole('alert');
    expect(alert).toHaveTextContent('Requerido');
  });

  it('does not render any alert when error is omitted', () => {
    render(<Field label="Email">{<input />}</Field>);
    expect(screen.queryByRole('alert')).toBeNull();
  });

  it('renders children as the form control', () => {
    render(<Field label="Email">{<input data-testid="my-input" />}</Field>);
    expect(screen.getByTestId('my-input')).toBeInTheDocument();
  });
});
