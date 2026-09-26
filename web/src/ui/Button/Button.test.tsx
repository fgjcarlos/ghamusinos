// TDD contract for Button (issue 154).
// Tests check the observable contract, not the CSS values.

import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { Button } from './Button';

describe('Button', () => {
  it('renders children inside a <button type="button"> by default', () => {
    render(<Button>Hola</Button>);
    const btn = screen.getByRole('button', { name: 'Hola' });
    expect(btn).toBeInTheDocument();
    expect(btn.tagName).toBe('BUTTON');
    expect(btn).toHaveAttribute('type', 'button');
  });

  it('fires onClick when clicked', () => {
    const onClick = vi.fn();
    render(<Button onClick={onClick}>Pulsa</Button>);
    fireEvent.click(screen.getByRole('button'));
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it('does not fire onClick when disabled', () => {
    const onClick = vi.fn();
    render(
      <Button disabled onClick={onClick}>
        No
      </Button>,
    );
    fireEvent.click(screen.getByRole('button'));
    expect(onClick).not.toHaveBeenCalled();
  });

  it('respects type="submit" when explicitly passed', () => {
    render(<Button type="submit">Enviar</Button>);
    expect(screen.getByRole('button')).toHaveAttribute('type', 'submit');
  });

  it('applies a distinct class per variant', () => {
    const { rerender } = render(<Button variant="primary">A</Button>);
    const primaryClass = screen.getByRole('button').className;
    rerender(<Button variant="secondary">A</Button>);
    const secondaryClass = screen.getByRole('button').className;
    rerender(<Button variant="destructive">A</Button>);
    const destructiveClass = screen.getByRole('button').className;
    rerender(<Button variant="disabled">A</Button>);
    const disabledClass = screen.getByRole('button').className;
    // Each variant maps to a unique class (Record<Variant, string>).
    expect(new Set([primaryClass, secondaryClass, destructiveClass, disabledClass]).size).toBe(4);
  });
});
