// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// useTheme — Open Design redesign (B1). Owns the light/dark theme
// attribute: reads `sf.theme` from localStorage (default 'dark'),
// mirrors it onto <html data-theme="..."> so the :root[data-theme]
// override block in theme.css retints every token, and persists on
// toggle. index.html sets the same attribute pre-paint to avoid a
// FOUC; this hook is the React-side source of truth after mount.
import { useCallback, useEffect, useState } from 'react';

export type Theme = 'dark' | 'light';

const STORAGE_KEY = 'sf.theme';

function readStoredTheme(): Theme {
  try {
    const v = localStorage.getItem(STORAGE_KEY);
    return v === 'light' ? 'light' : 'dark';
  } catch {
    return 'dark';
  }
}

function applyTheme(theme: Theme) {
  document.documentElement.dataset.theme = theme;
}

export function useTheme() {
  const [theme, setTheme] = useState<Theme>(readStoredTheme);

  useEffect(() => {
    applyTheme(theme);
    try {
      localStorage.setItem(STORAGE_KEY, theme);
    } catch {
      // storage unavailable (private mode) — session-only theme
    }
  }, [theme]);

  const toggleTheme = useCallback(() => {
    setTheme((t) => (t === 'dark' ? 'light' : 'dark'));
  }, []);

  return { theme, toggleTheme };
}
