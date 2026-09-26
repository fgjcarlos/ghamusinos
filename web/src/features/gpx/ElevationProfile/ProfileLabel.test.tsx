// TDD contract for ProfileLabel (issue 156).

import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/react';
import { ProfileLabel } from './ProfileLabel';
import type { ProjectedBand } from './projectTrack';

describe('ProfileLabel', () => {
  it('renders the children text in an SVG <text>', () => {
    const band: ProjectedBand = { x: 50, width: 100, type: 'climb' };
    const { container } = render(
      <ProfileLabel band={band} y={30}>
        Subida reina
      </ProfileLabel>,
    );
    const text = container.querySelector('text');
    expect(text).toHaveTextContent('Subida reina');
  });

  it('centers the text horizontally over the band', () => {
    const band: ProjectedBand = { x: 40, width: 80, type: 'climb' };
    const { container } = render(
      <ProfileLabel band={band} y={20}>
        Hello
      </ProfileLabel>,
    );
    const text = container.querySelector('text');
    const expectedCenterX = band.x + band.width / 2;
    expect(text?.getAttribute('x')).toBe(String(expectedCenterX));
  });

  it('sits the text just above the band (y - 4)', () => {
    const band: ProjectedBand = { x: 0, width: 50, type: 'risk' };
    const { container } = render(
      <ProfileLabel band={band} y={50}>
        Above
      </ProfileLabel>,
    );
    const text = container.querySelector('text');
    expect(text?.getAttribute('y')).toBe('46');
  });
});
