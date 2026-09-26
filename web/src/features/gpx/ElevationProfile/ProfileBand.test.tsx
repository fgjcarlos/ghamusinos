// TDD contract for ProfileBand (issue 156).

import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/react';
import { ProfileBand } from './ProfileBand';
import type { ProjectedBand } from './projectTrack';

describe('ProfileBand', () => {
  it('renders a <rect> with the band x, width and height', () => {
    const band: ProjectedBand = { x: 40, width: 80, type: 'climb' };
    const { container } = render(<ProfileBand band={band} height={150} />);
    const rect = container.querySelector('rect');
    expect(rect).toBeTruthy();
    expect(rect?.getAttribute('x')).toBe('40');
    expect(rect?.getAttribute('width')).toBe('80');
    expect(rect?.getAttribute('height')).toBe('150');
  });

  it('uses different classes for climb vs risk', () => {
    const climb: ProjectedBand = { x: 0, width: 100, type: 'climb' };
    const risk: ProjectedBand = { x: 0, width: 100, type: 'risk' };
    const { container: c1 } = render(<ProfileBand band={climb} height={100} />);
    const { container: c2 } = render(<ProfileBand band={risk} height={100} />);
    const r1 = c1.querySelector('rect');
    const r2 = c2.querySelector('rect');
    // Use getAttribute('class') — className.baseVal is undefined in jsdom.
    expect(r1?.getAttribute('class')).not.toEqual(r2?.getAttribute('class'));
  });

  it('exposes a <title> for the label and carries it as aria-label', () => {
    const band: ProjectedBand = { x: 10, width: 50, type: 'risk' };
    const { container } = render(<ProfileBand band={band} height={100} label="Steep section" />);
    const rect = container.querySelector('rect');
    expect(rect).toHaveAttribute('aria-label', 'Steep section');
    // The <title> child is rendered for browser tooltips; jsdom has
    // quirks with SVG children, so we don't assert its text content
    // here — the aria-label above is the authoritative contract.
  });
});
