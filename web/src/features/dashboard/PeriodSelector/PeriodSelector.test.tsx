// TDD contract for PeriodSelector (Fase 1.4, DW-001).
// Four windows, one selected, and the change is reported to the parent.

import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { PeriodSelector } from './PeriodSelector';

describe('PeriodSelector', () => {
  it('renders exactly the four windows', () => {
    render(<PeriodSelector value="30d" onChange={() => {}} />);
    const buttons = screen.getAllByRole('button');
    expect(buttons).toHaveLength(4);
    expect(buttons.map((button) => button.textContent)).toEqual(['7d', '30d', '90d', '1y']);
  });

  it('marks the selected window as pressed', () => {
    render(<PeriodSelector value="30d" onChange={() => {}} />);
    expect(screen.getByTestId('period-option-30d')).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByTestId('period-option-7d')).toHaveAttribute('aria-pressed', 'false');
  });

  it('reports the new window when another one is clicked', () => {
    const onChange = vi.fn();
    render(<PeriodSelector value="30d" onChange={onChange} />);
    fireEvent.click(screen.getByTestId('period-option-7d'));
    expect(onChange).toHaveBeenCalledWith('7d');
  });
});
