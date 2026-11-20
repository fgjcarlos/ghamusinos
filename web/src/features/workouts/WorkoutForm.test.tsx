import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';
import { WorkoutForm } from './WorkoutForm';
import type { WorkoutInput } from '../../lib/api/workout-types';

describe('WorkoutForm', () => {
  it('rejects dates outside the displayed civil-date range', async () => {
    const onSave = vi.fn<(value: WorkoutInput) => Promise<void>>().mockResolvedValue();
    render(
      <WorkoutForm
        onSave={onSave}
        onCancel={vi.fn()}
        busy={false}
        dateFrom="2026-11-16"
        dateTo="2027-11-16"
      />,
    );
    fireEvent.change(document.querySelector<HTMLInputElement>('input[name="scheduled_on"]')!, {
      target: { value: '2026-11-15' },
    });
    fireEvent.change(document.querySelector<HTMLInputElement>('input[name="title"]')!, {
      target: { value: 'Sesión' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Guardar' }));
    expect(
      await screen.findByText(/La fecha debe estar entre 2026-11-16 y 2027-11-16/),
    ).toBeInTheDocument();
    expect(onSave).not.toHaveBeenCalled();
  });

  it('associates validation messages with inputs and preserves zero while nulling empty targets', async () => {
    const onSave = vi.fn<(value: WorkoutInput) => Promise<void>>().mockResolvedValue();
    render(<WorkoutForm onSave={onSave} onCancel={vi.fn()} busy={false} />);
    fireEvent.click(screen.getByRole('button', { name: 'Guardar' }));
    const date = document.querySelector<HTMLInputElement>('input[name="scheduled_on"]')!;
    expect(date).toHaveAttribute('aria-invalid', 'true');
    expect(date).toHaveAttribute('aria-describedby');
    expect(await screen.findByText('El título es obligatorio.')).toBeInTheDocument();

    fireEvent.change(date, { target: { value: '2026-12-01' } });
    fireEvent.change(document.querySelector<HTMLInputElement>('input[name="title"]')!, {
      target: { value: 'Entreno' },
    });
    fireEvent.change(
      document.querySelector<HTMLInputElement>('input[name="target_duration_sec"]')!,
      { target: { value: '0' } },
    );
    fireEvent.click(screen.getByRole('button', { name: 'Guardar' }));
    expect(onSave).toHaveBeenCalledWith(
      expect.objectContaining({
        target_duration_sec: 0,
        target_distance_m: null,
        target_elevation_gain_m: null,
      }),
    );
  });

  it('rejects a numeric input with native badInput instead of clearing its target', async () => {
    const onSave = vi.fn<(value: WorkoutInput) => Promise<void>>().mockResolvedValue();
    render(<WorkoutForm onSave={onSave} onCancel={vi.fn()} busy={false} />);
    fireEvent.change(document.querySelector<HTMLInputElement>('input[name="scheduled_on"]')!, {
      target: { value: '2026-12-01' },
    });
    fireEvent.change(document.querySelector<HTMLInputElement>('input[name="title"]')!, {
      target: { value: 'Entreno' },
    });
    const duration = document.querySelector<HTMLInputElement>('input[name="target_duration_sec"]')!;
    Object.defineProperty(duration, 'validity', {
      configurable: true,
      value: { badInput: true },
    });

    fireEvent.click(screen.getByRole('button', { name: 'Guardar' }));

    expect(await screen.findByText('Introduce un número válido.')).toBeInTheDocument();
    expect(duration).toHaveAttribute('aria-invalid', 'true');
    expect(onSave).not.toHaveBeenCalled();
  });
});
