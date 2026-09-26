// Smoke test: validates the test runner pipeline end-to-end.
// React 19 + jsdom + @testing-library/react + jest-dom matchers.

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';

describe('Smoke', () => {
  it('renders React in jsdom with jest-dom matchers', () => {
    render(<div data-testid="hello">Hola</div>);
    expect(screen.getByTestId('hello')).toHaveTextContent('Hola');
  });
});
