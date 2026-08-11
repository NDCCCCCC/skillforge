// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Vite config for the Phase 1 spike stub. In dev, the `/api` prefix is
// proxied to the Hertz backend on :8888 so the browser sees a single
// origin (no CORS). In production, SkillForge serves the built frontend
// and the API from the same origin (single-origin deployment).
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:8888',
        changeOrigin: true,
      },
    },
  },
});
