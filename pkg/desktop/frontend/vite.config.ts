import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  base: './',
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    target: 'es2022',
    assetsInlineLimit: 4096,
    sourcemap: false,
    reportCompressedSize: false,
  },
  server: {
    port: 34115,
    strictPort: true,
    fs: {
      allow: ['..'],
    },
  },
});
