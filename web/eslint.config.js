// ESLint flat config para web/. Stack: React 19 + TS 6 + Vite 8.
// El plugin react-refresh asume que existe un solo archivo que se
// ejecuta como entry (src/main.tsx); si en el futuro hay varios
// entries, ajustar los patterns. Issue #35.

import js from '@eslint/js';
import reactHooks from 'eslint-plugin-react-hooks';
import reactRefresh from 'eslint-plugin-react-refresh';
import prettier from 'eslint-config-prettier';

export default [
  // Base recomendada para cualquier proyecto TS
  js.configs.recommended,

  // Reglas específicas de React (hooks + refresh)
  {
    plugins: {
      'react-hooks': reactHooks,
      'react-refresh': reactRefresh,
    },
    languageOptions: {
      // Tipos del DOM + ES2020 (mismo nivel que tsconfig.json)
      globals: {
        // Si se necesitan más, importarlos de globals o listarlos aquí.
        document: 'readonly',
        window: 'readonly',
      },
      parserOptions: {
        ecmaVersion: 2020,
        sourceType: 'module',
      },
    },
    rules: {
      ...reactHooks.configs.recommended.rules,
      // react-refresh: sólo exporta componentes desde archivos .tsx
      // que se usan como punto de entrada o que no se exportan en
      // absoluto. El "warn" es deliberado: a veces exportamos
      // helpers y queremos que sólo avise, no rompa la CI, hasta que
      // definamos una convención más estricta en una issue separada.
      'react-refresh/only-export-components': 'warn',
    },
  },

  // Prettier al final para desactivar reglas de estilo que ya
  // controla el formateador.
  prettier,
];
