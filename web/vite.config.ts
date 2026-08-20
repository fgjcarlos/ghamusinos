import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  build: {
    // El output cae dentro del paquete Go que lo embebe.
    // go:embed no admite rutas con '..', así que la carpeta dist
    // debe estar dentro del módulo Go (internal/frontend/dist).
    outDir: '../internal/frontend/dist',
    emptyOutDir: true,
  },
});
