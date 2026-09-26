// TDD contract for MetricTile (issue 154).

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MetricTile } from './MetricTile';

describe('MetricTile', () => {
  it('renders the label as visible small text', () => {
    render(<MetricTile label="Distancia" value="10" />);
    expect(screen.getByText('Distancia')).toBeInTheDocument();
  });

  it('renders the value as the dominant element', () => {
    render(<MetricTile label="Distancia" value="10" />);
    expect(screen.getByText('10')).toBeInTheDocument();
  });

  it('renders the unit suffix next to the value when provided', () => {
    render(<MetricTile label="Tiempo" value="2:30" unit="h" />);
    // Value and unit are sibling text nodes inside valueRow; a
    // regex cannot span element boundaries, so use a function
    // matcher against the combined textContent.
    expect(screen.getByText((_, el) => el?.textContent === '2:30h')).toBeInTheDocument();
  });

  it('accepts a numeric value', () => {
    render(<MetricTile label="Pasos" value={1234} />);
    expect(screen.getByText('1234')).toBeInTheDocument();
  });
});
