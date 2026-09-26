// TDD contract for HRZoneBars (issue 158).
// Activities without HR data (no pulsometer) render "Sin FC" instead
// of five zero bars. With HR data, render up to 5 bars whose height
// is proportional to the value (in % time-in-zone).

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { HRZoneBars } from './HRZoneBars';

describe('HRZoneBars', () => {
  it('renders "Sin FC" when zones is empty', () => {
    render(<HRZoneBars zones={[]} />);
    expect(screen.getByText('Sin FC')).toBeInTheDocument();
  });

  it('renders "Sin FC" when zones is undefined', () => {
    render(<HRZoneBars />);
    expect(screen.getByText('Sin FC')).toBeInTheDocument();
  });

  it('renders 5 bars when zones has 5 elements', () => {
    const { container } = render(<HRZoneBars zones={[10, 20, 30, 25, 15]} />);
    const bars = container.querySelectorAll('[aria-label^="Zona "]');
    expect(bars.length).toBe(5);
  });

  it('renders only the provided zones when fewer than 5', () => {
    const { container } = render(<HRZoneBars zones={[10, 20]} />);
    const bars = container.querySelectorAll('[aria-label^="Zona "]');
    expect(bars.length).toBe(2);
  });

  it('caps at 5 bars when zones has more than 5', () => {
    const { container } = render(<HRZoneBars zones={[1, 2, 3, 4, 5, 6, 7]} />);
    const bars = container.querySelectorAll('[aria-label^="Zona "]');
    expect(bars.length).toBe(5);
  });

  it('does not render "Sin FC" when zones has at least one value', () => {
    render(<HRZoneBars zones={[10, 0, 0, 0, 0]} />);
    expect(screen.queryByText('Sin FC')).toBeNull();
  });
});
