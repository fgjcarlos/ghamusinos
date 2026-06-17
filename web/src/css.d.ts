// Type declarations for non-TS imports in this app.
// TypeScript 6.x rejects side-effect imports of `.css` without an
// ambient declaration; this file satisfies that rule without touching
// the build pipeline (Vite handles CSS at build time).
declare module '*.css';
