import { describe, expect, it } from 'vitest';
import topBarCss from './TopBar.module.css?raw';
import shellCss from './AppShell.module.css?raw';
import workoutsCss from '../../features/workouts/Workouts.module.css?raw';

describe('mobile layout styles', () => {
  it('hides the desktop top navigation at the bottom-nav breakpoint', () => {
    expect(topBarCss).toMatch(
      /@media\s*\(max-width:\s*767px\)[\s\S]*?\.nav\s*\{[^}]*display:\s*none/,
    );
  });

  it('allows the four bottom links and long workout content to shrink and wrap', () => {
    expect(shellCss).toMatch(/\.bottomNav\s*>\s*a\s*\{[^}]*flex:\s*1\s+1\s+0/);
    expect(shellCss).toMatch(/\.bottomNav\s*>\s*a\s*\{[^}]*min-width:\s*0/);
    expect(workoutsCss).toMatch(/\.sessionActions\s*\{[^}]*flex-wrap:\s*wrap/);
    expect(workoutsCss).toMatch(/\.sessionActions\s*>\s*button\s*\{[^}]*overflow-wrap:\s*anywhere/);
    expect(workoutsCss).toMatch(/\.notes\s*\{[^}]*overflow-wrap:\s*anywhere/);
    expect(workoutsCss).toMatch(/\.sessionHeading\s+h3\s*\{[^}]*overflow-wrap:\s*anywhere/);
  });
});
