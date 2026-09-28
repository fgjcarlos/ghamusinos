import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ComparisonElevationProfile } from './ComparisonElevationProfile';

const points = Array.from({ length: 10 }, (_, i) => ({
  lat: 40.4 + i * 0.001,
  lng: -3.7 + i * 0.001,
  ele: 1000 + i * 10,
}));

describe('ComparisonElevationProfile', () => {
  it('projects populated tracks without NaN values', () => {
    render(
      <ComparisonElevationProfile
        tracks={[
          { name: 'A', points },
          { name: 'B', points },
          { name: 'C', points },
        ]}
      />,
    );
    const profile = screen.getByTestId('comparison-elevation-profile');
    expect(profile.querySelectorAll('svg')).toHaveLength(3);
    expect(profile.innerHTML).not.toContain('NaN');
    profile.querySelectorAll('svg').forEach((svg) => {
      const path = svg.querySelector('path:nth-of-type(2)')?.getAttribute('d') ?? '';
      expect(path.match(/L/g)?.length).toBeGreaterThanOrEqual(9);
      expect(path).not.toContain('NaN');
    });
  });
});
