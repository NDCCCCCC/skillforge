// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Vitest config — shares the Vite react plugin so .tsx transforms
// identically in `npm test` and `npm run dev`. jsdom environment gives
// @testing-library/react a DOM to render into.
import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';
import path from 'node:path';

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./vitest.setup.ts'],
    include: ['src/**/*.{test,spec}.{ts,tsx}'],
  },
});
