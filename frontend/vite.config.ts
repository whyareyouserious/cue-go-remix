import { defineConfig } from 'vite';

export default defineConfig({
  base: './',
  build: { target: 'es2022', rollupOptions: { external: ['/wails/runtime.js'] } },
  server: { strictPort: true, port: 5173 },
});
