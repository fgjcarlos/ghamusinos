// Runtime resolution of the design tokens (web/src/styles/tokens.css).
//
// ECharts paints on a canvas, so it cannot consume `var(--gh-*)` the
// way CSS can: the token values have to be resolved before they reach
// the chart option. The helper reads the custom property from the
// document root and falls back to 'transparent' when the value is not
// available (jsdom does not apply the stylesheet, and a transparent
// chart is a far better failure mode than a hardcoded colour).

export const FALLBACK_COLOR = 'transparent';

/**
 * resolveToken returns the value of a CSS custom property defined on
 * :root, or 'transparent' when it cannot be resolved.
 */
export function resolveToken(name: string): string {
  if (typeof window === 'undefined' || typeof document === 'undefined') {
    return FALLBACK_COLOR;
  }
  const raw = window.getComputedStyle(document.documentElement).getPropertyValue(name);
  const value = raw.trim();
  return value.length > 0 ? value : FALLBACK_COLOR;
}
