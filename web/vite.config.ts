// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Vite config for the SkillForge Streaming Trust UI (Phase 4). In dev,
// the `/api` prefix is proxied to the Hertz backend on :8888 so the
// browser sees a single origin (no CORS). In production, SkillForge
// serves the built frontend and the API from the same origin
// (single-origin deployment).
//
// Tailwind v4 plugin registered so `theme.css` compiles via
// `@tailwindcss/vite` and the `@theme { ... }` token block generates
// the utility classes (bg-primary, text-status-compliant, etc.). Path
// alias `@` → `src` so Wave 2+ imports can use `@/components/...`.
import path from 'node:path';
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:8888',
        changeOrigin: true,
      },
    },
  },
});
