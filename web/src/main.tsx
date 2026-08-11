// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Phase 1 spike stub entrypoint. Mounts <App /> into #root. StrictMode
// double-invokes effects in dev to surface hidden side effects early;
// Phase 4's real UI follows the same mounting pattern.
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import App from './App';

const rootEl = document.getElementById('root');
if (!rootEl) {
  throw new Error('SkillForge spike stub: #root element not found in index.html');
}

createRoot(rootEl).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
