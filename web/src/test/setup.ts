// Loaded once before each test file.
// Wires jest-dom custom matchers (toBeInTheDocument, toHaveAccessibleName, ...)
// onto Vitest's expect, and runs cleanup() after each test so the next render
// starts from an empty jsdom.
//
// Auto-cleanup from @testing-library/react v16 needs explicit registration
// in Vitest: registering it here once is simpler than calling cleanup() in
// every test.
//
// Extensible: añadir polyfills globales (ResizeObserver para jsdom) o setup
// de MSW cuando lleguen tests de API.

import { afterEach } from 'vitest';
import { cleanup } from '@testing-library/react';
import '@testing-library/jest-dom/vitest';

afterEach(() => {
  cleanup();
});
