// TDD contract for SeverityPill (issue 154).
// Severity is NEVER communicated by color alone: the level is
// always rendered as visible Spanish text.

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { SeverityPill } from './SeverityPill';

describe('SeverityPill', () => {
  it('renders Spanish text for medium', () => {
    render(<SeverityPill level="medium" />);
    expect(screen.getByText(/Riesgo medio/i)).toBeInTheDocument();
  });

  it('renders Spanish text for high', () => {
    render(<SeverityPill level="high" />);
    expect(screen.getByText(/Riesgo alto/i)).toBeInTheDocument();
  });

  it('uses a different CSS class per level', () => {
    const { rerender } = render(<SeverityPill level="medium" />);
    const mediumClass = screen.getByTestId('severity-medium').className;
    rerender(<SeverityPill level="high" />);
    const highClass = screen.getByTestId('severity-high').className;
    expect(mediumClass).not.toBe(highClass);
  });

  it('exposes the level via aria-label', () => {
    render(<SeverityPill level="high" />);
    const el = screen.getByTestId('severity-high');
    expect(el).toHaveAccessibleName(/Riesgo alto/i);
  });
});
