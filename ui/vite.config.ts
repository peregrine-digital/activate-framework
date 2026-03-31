import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import tailwindcss from '@tailwindcss/vite';
import { resolve } from 'path';

export default defineConfig(({ mode }) => ({
  plugins: [svelte(), tailwindcss()],
  build: mode === 'webview'
    ? {
        // Webview build: single JS+CSS bundle for VS Code extension
        outDir: resolve(__dirname, '../extension/webview-dist'),
        emptyOutDir: true,
        rollupOptions: {
          input: resolve(__dirname, 'src/webview.ts'),
          output: {
            entryFileNames: 'webview.js',
            assetFileNames: 'webview.[ext]',
          },
        },
      }
    : {
        // Default build: dev preview app
        outDir: 'dist',
      },
}));

