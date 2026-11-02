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
    expect(screen.getByText(/hr_max must be between 1 and 260/i)).toBeInTheDocument();
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
    expect(screen.getByRole('button', { name: /guardando/i })).toBeDisabled();
  });

  // ─────────────────────────────────────────────────────────────────
  // Aviso de transferencia a proveedor de IA externo (fase 1.5, T2).
  // ─────────────────────────────────────────────────────────────────

  it('shows the transfer notice naming the external AI provider', () => {
    render(
      <PreferencesForm
        values={makeValues()}
        errors={{}}
        busy={false}
        onChange={() => {}}
        onSubmit={() => {}}
      />,
    );
    const notice = screen.getByTestId('ai-consent-notice');
    expect(notice).toBeInTheDocument();
    expect(notice.textContent ?? '').toMatch(/proveedor de IA externo/i);
  });

  it('lists only the aggregate metrics that may leave the system', () => {
    render(
      <PreferencesForm
        values={makeValues()}
        errors={{}}
        busy={false}
        onChange={() => {}}
        onSubmit={() => {}}
      />,
    );
    const text = screen.getByTestId('ai-consent-notice').textContent ?? '';
    expect(text).toMatch(/distancia/i);
    expect(text).toMatch(/duración/i);
    expect(text).toMatch(/desnivel/i);
    expect(text).toMatch(/frecuencia cardíaca/i);
    expect(text).toMatch(/carga calculada/i);
  });

  it('states that no identifying data, GPS coordinates or raw streams are sent', () => {
    render(
      <PreferencesForm
        values={makeValues()}
        errors={{}}
        busy={false}
        onChange={() => {}}
        onSubmit={() => {}}
      />,
    );
    const text = screen.getByTestId('ai-consent-notice').textContent ?? '';
    expect(text).toMatch(/no se env/i);
    expect(text).toMatch(/identific/i);
    expect(text).toMatch(/nombre/i);
    expect(text).toMatch(/correo/i);
    expect(text).toMatch(/GPS/i);
    expect(text).toMatch(/streams crudos/i);
  });

  it('renders the AI consent checkbox unchecked when there is no consent', () => {
    render(
      <PreferencesForm
        values={makeValues({ ai_enabled: false })}
        errors={{}}
        busy={false}
        onChange={() => {}}
        onSubmit={() => {}}
      />,
    );
    expect(screen.getByRole('checkbox', { name: /proveedor de IA/i })).not.toBeChecked();
  });

  it('emits onChange with true only when the user checks the consent box', () => {
    const onChange = vi.fn();
    render(
      <PreferencesForm
        values={makeValues({ ai_enabled: false })}
        errors={{}}
        busy={false}
        onChange={onChange}
        onSubmit={() => {}}
      />,
    );
    fireEvent.click(screen.getByRole('checkbox', { name: /proveedor de IA/i }));
    expect(onChange).toHaveBeenCalledWith('ai_enabled', true);
  });

  it('emits onChange with false when the user revokes the consent', () => {
    const onChange = vi.fn();
    render(
      <PreferencesForm
        values={makeValues({ ai_enabled: true })}
        errors={{}}
        busy={false}
        onChange={onChange}
        onSubmit={() => {}}
      />,
    );
    fireEvent.click(screen.getByRole('checkbox', { name: /proveedor de IA/i }));
    expect(onChange).toHaveBeenCalledWith('ai_enabled', false);
  });
});
