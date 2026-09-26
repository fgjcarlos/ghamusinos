// Vitest configuration for the web app.
// Mirrors the build target (Vite + React) but runs tests in a jsdom
// environment, with @testing-library/jest-dom matchers wired in via
// src/test/setup.ts.
//
// Imports de `vitest` se hacen explícitos en cada archivo `*.test.tsx`
// para no contaminar el ámbito global y mantener `tsconfig` libre de
// `types: ["vitest/globals"]`. Mismo idioma que el resto de la base.

import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    css: true,
  },
});
