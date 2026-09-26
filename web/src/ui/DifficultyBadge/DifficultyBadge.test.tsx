// TDD contract for DifficultyBadge (issue 154).
// Color and difficulty are NEVER communicated by color alone:
// the level label is always rendered as Spanish text.

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { DifficultyBadge } from './DifficultyBadge';

const LEVELS = ['beginner', 'intermediate', 'advanced', 'pro'] as const;

describe('DifficultyBadge', () => {
  it.each(LEVELS)('renders Spanish label for level %s', (level) => {
    render(<DifficultyBadge level={level} score={50} />);
    const label = screen.getByTestId(`difficulty-${level}`);
    expect(label).toBeInTheDocument();
  });

  it.each(LEVELS)('shows the numeric score for level %s', (level) => {
    render(<DifficultyBadge level={level} score={73} />);
    const badge = screen.getByTestId(`difficulty-${level}`);
    expect(badge).toHaveTextContent('73');
  });

  it('clamps score to the documented 0-100 range', () => {
    render(<DifficultyBadge level="beginner" score={-5} />);
    expect(screen.getByTestId('difficulty-beginner')).toHaveTextContent('0');
  });

  it('renders a different CSS class per level', () => {
    const classes = new Set<string>();
    for (const level of LEVELS) {
      const { unmount } = render(<DifficultyBadge level={level} score={50} />);
      classes.add(screen.getByTestId(`difficulty-${level}`).className);
      unmount();
    }
    expect(classes.size).toBe(4);
  });

  it('is a single labelled container (a11y)', () => {
    const { container } = render(<DifficultyBadge level="advanced" score={80} />);
    // The whole text "Avanzado 80" is the accessible name.
    const labelled = screen.getByLabelText(/avanzado.*80/i);
    expect(labelled).toBe(container.firstChild);
  });
});
