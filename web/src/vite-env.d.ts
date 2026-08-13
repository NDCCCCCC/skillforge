// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

/// <reference types="vite/client" />

// Project-local Vite env vars. Only names prefixed with VITE_ are exposed
// to the browser via import.meta.env. vite/client already declares the
// built-ins (PROD, DEV, MODE, BASE_URL, SSR); this block merges in our own.
interface ImportMetaEnv {
  /**
   * "1" → dev server points at the live Hertz backend (DefaultChatTransport
   * via the vite /api→:8888 proxy) instead of MockChatTransport. Unset →
   * Mock (the default, keeps `npm run dev` + vitest backend-free). See
   * web/.env.example for usage (Git Bash / PowerShell / .env.local).
   */
  readonly VITE_REAL_BACKEND?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
