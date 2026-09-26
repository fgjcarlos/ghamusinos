// Loaded once before each test file.
// Wires jest-dom custom matchers (toBeInTheDocument, toHaveAccessibleName, ...)
// onto Vitest's expect.
//
// Extensible: aquí se pueden añadir polyfills globales (e.g. ResizeObserver
// for jsdom) o setup de MSW cuando lleguen tests de API.

import '@testing-library/jest-dom';
