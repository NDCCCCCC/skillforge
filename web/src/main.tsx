// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Phase 4 entrypoint. Mounts <App /> into #root inside React 19's
// StrictMode (double-invokes effects in dev to surface hidden side
// effects early). Imports fontsource variables and `theme.css` BEFORE
// the React render so the design tokens (Inter Variable, JetBrains
// Mono Variable, status colors, spacing scale) are applied to the
// entire DOM tree before any component paints.
import '@fontsource-variable/inter';
import '@fontsource-variable/jetbrains-mono';
import './styles/theme.css';

import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import App from './App';

const rootEl = document.getElementById('root');
if (!rootEl) {
  throw new Error('SkillForge: #root element not found in index.html');
}

createRoot(rootEl).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
