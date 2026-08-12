// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 05-03 Task 2 — StatusBar tests.
//
// Pins the SC-2 pinned-version indicator acceptance assertions:
//   1. Renders the inherited idle label `就绪` when no pinned version.
//   2. Renders the pinned-version label when pinnedVersion=true.
//   3. data-testid attributes expose the pinned state for UAT.
import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import '@testing-library/jest-dom/vitest';
import StatusBar from '../components/status/StatusBar';

afterEach(cleanup);

describe('StatusBar', () => {
  it('renders the inherited idle label 就绪 by default', () => {
    render(<StatusBar />);
    expect(screen.getByTestId('statusbar-idle')).toBeInTheDocument();
    expect(screen.getByTestId('statusbar-idle')).toHaveTextContent('就绪');
  });

  it('renders the pinned-version indicator when pinnedVersion=true', () => {
    render(<StatusBar pinnedVersion />);
    expect(screen.getByTestId('statusbar-pinned')).toBeInTheDocument();
    expect(screen.getByTestId('statusbar-pinned')).toHaveTextContent('当前运行使用旧版本');
  });

  it('renders the pinned-version indicator with the skill name when provided', () => {
    render(<StatusBar pinnedVersion pinnedSkillName="compliance-review" />);
    expect(screen.getByTestId('statusbar-pinned')).toHaveTextContent('当前运行使用旧版本（compliance-review）');
  });

  it('exposes the pinned state via data-pinned attribute', () => {
    const { rerender } = render(<StatusBar />);
    expect(screen.getByTestId('app-statusbar')).toHaveAttribute('data-pinned', 'false');
    rerender(<StatusBar pinnedVersion />);
    expect(screen.getByTestId('app-statusbar')).toHaveAttribute('data-pinned', 'true');
  });
});
