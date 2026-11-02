import { mkdirSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

// The Go binary embeds this directory (see internal/webui).
const outDir = resolve(__dirname, '../internal/webui/dist');

export default defineConfig({
  plugins: [
    svelte(),
    {
      // Vite empties outDir on every build. Keep the placeholder that lets the
      // Go package compile before the frontend has been built.
      name: 'keep-dist-placeholder',
      closeBundle() {
        mkdirSync(outDir, { recursive: true });
        writeFileSync(resolve(outDir, '.gitkeep'), '');
      },
    },
  ],
  build: {
    outDir,
    emptyOutDir: true,
    target: 'es2022',
  },
  server: {
    // `make dev` runs the Go server on :8080 and Vite on :5173.
    proxy: {
      '/healthz': 'http://127.0.0.1:8080',
      '/api': 'http://127.0.0.1:8080',
    },
  },
});
