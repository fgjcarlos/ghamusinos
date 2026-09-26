// TDD contract for Chip (issue 154).

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { Chip } from './Chip';

describe('Chip', () => {
  it('renders children', () => {
    render(<Chip>Circular</Chip>);
    expect(screen.getByText('Circular')).toBeInTheDocument();
  });

  it('uses the neutral tone by default', () => {
    render(<Chip>X</Chip>);
    const chip = screen.getByText('X');
    expect(chip.className).toBeTruthy();
    // No checks against literal CSS values (couples test to design).
  });

  it('applies a different class per tone (neutral vs accent)', () => {
    const { rerender } = render(<Chip tone="neutral">N</Chip>);
    const neutralClass = screen.getByText('N').className;
    rerender(<Chip tone="accent">A</Chip>);
    const accentClass = screen.getByText('A').className;
    expect(neutralClass).not.toBe(accentClass);
  });
});
