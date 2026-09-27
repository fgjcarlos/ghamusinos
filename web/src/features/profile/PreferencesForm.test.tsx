// TDD contract for PreferencesForm (issue 159).

import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { PreferencesForm } from './PreferencesForm';
import type { PreferencesFormValues } from './PreferencesForm';

function makeValues(overrides: Partial<PreferencesFormValues> = {}): PreferencesFormValues {
  return {
    hr_max: 180,
    lthr: 150,
    ftp: 280,
    level: 'intermediate',
    timezone: 'Europe/Madrid',
    ai_enabled: true,
    ...overrides,
  };
}

describe('PreferencesForm', () => {
  it('renders the timezone as required', () => {
    render(
      <PreferencesForm
        values={makeValues()}
        errors={{}}
        busy={false}
        onChange={() => {}}
        onSubmit={() => {}}
      />,
    );
    expect(screen.getByLabelText(/Zona horaria/i)).toBeInTheDocument();
  });

  it('shows an error message next to the field that failed', () => {
    render(
      <PreferencesForm
        values={makeValues()}
        errors={{ hr_max: 'hr_max must be between 1 and 260 (or null)' }}
        busy={false}
        onChange={() => {}}
        onSubmit={() => {}}
      />,
    );
    expect(
      screen.getByText(/hr_max must be between 1 and 260/i),
    ).toBeInTheDocument();
  });

  it('renders the cross-field warning when present', () => {
    render(
      <PreferencesForm
        values={makeValues()}
        errors={{}}
        warning="lthr is greater than hr_max — verify the readings"
        busy={false}
        onChange={() => {}}
        onSubmit={() => {}}
      />,
    );
    expect(screen.getByText(/lthr is greater than hr_max/i)).toBeInTheDocument();
  });

  it('emits onChange with the parsed integer when hr_max changes', () => {
    const onChange = vi.fn();
    render(
      <PreferencesForm
        values={makeValues()}
        errors={{}}
        busy={false}
        onChange={onChange}
        onSubmit={() => {}}
      />,
    );
    const hrMaxInput = screen.getByLabelText(/FC máxima/i);
    fireEvent.change(hrMaxInput, { target: { value: '190' } });
    expect(onChange).toHaveBeenCalledWith('hr_max', 190);
  });

  it('emits onChange with null when hr_max is cleared', () => {
    const onChange = vi.fn();
    render(
      <PreferencesForm
        values={makeValues()}
        errors={{}}
        busy={false}
        onChange={onChange}
        onSubmit={() => {}}
      />,
    );
    const hrMaxInput = screen.getByLabelText(/FC máxima/i);
    fireEvent.change(hrMaxInput, { target: { value: '' } });
    expect(onChange).toHaveBeenCalledWith('hr_max', null);
  });

  it('emits onChange when level changes', () => {
    const onChange = vi.fn();
    render(
      <PreferencesForm
        values={makeValues({ level: null })}
        errors={{}}
        busy={false}
        onChange={onChange}
        onSubmit={() => {}}
      />,
    );
    const levelSelect = screen.getByLabelText(/Nivel/i);
    fireEvent.change(levelSelect, { target: { value: 'advanced' } });
    expect(onChange).toHaveBeenCalledWith('level', 'advanced');
  });

  it('disables all inputs when busy', () => {
    render(
      <PreferencesForm
        values={makeValues()}
        errors={{}}
        busy
        onChange={() => {}}
        onSubmit={() => {}}
      />,
    );
    expect(
      screen.getByRole('button', { name: /guardando/i }),
    ).toBeDisabled();
  });
});
