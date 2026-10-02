// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// ThemeToggle — Open Design redesign B3. Formal home of the light/dark
// switch (the B1 interim button lived in the StatusBar). useTheme owns
// the sf.theme localStorage key + the <html data-theme> attribute; this
// component is presentation only.
import { Moon, Sun } from 'lucide-react';
import { useTheme } from '../../hooks/useTheme';

export default function ThemeToggle() {
  const { theme, toggleTheme } = useTheme();

  return (
    <button
      type="button"
      data-testid="theme-toggle"
      onClick={toggleTheme}
      className="inline-flex h-8 w-8 items-center justify-center rounded-md text-text-muted transition-colors hover:bg-surface-2 hover:text-text"
      aria-label={theme === 'dark' ? '切换到浅色主题' : '切换到暗色主题'}
      title={theme === 'dark' ? '切换到浅色主题' : '切换到暗色主题'}
    >
      {theme === 'dark' ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
    </button>
  );
}
