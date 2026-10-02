// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 05-03 Task 2 + Plan 05-07 Task 2 + Plan 05-08 Task 3 — SkillManagerMenu tests.
//
// Plan 05-07 (D5-IA-04 + D5-IA-05) reduces the dropdown to the wizard
// entry only:
//   - 全部重载 button (skill-manager-reload-all) REMOVED — D5-IA-05
//     deletes the dead POST /api/skills/reload route.
//   - Per-skill list (skill-manager-item rows + status badges) MOVED
//     to the LeftSidebar 技能 block per D5-IA-04 division of labor.
//   - The 新建 Skill（向导） entry (skill-manager-new-skill) remains
//     and is the sole entry point.
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import '@testing-library/jest-dom/vitest';
import SkillManagerMenu from '../components/layout/SkillManagerMenu';
import { useAppStore } from '../store/app-store';

const FROZEN_TS = 1700000000000;
const originalState = useAppStore.getState();

afterEach(() => {
  cleanup();
  // Reset every slice this test file touches.
  useAppStore.setState({
    ...originalState,
    skillReloadStatus: {},
    wizardMode: { active: false, step: 0, totalSteps: 0 },
    centerView: { kind: 'empty' },
    skills: [],
    sessions: [{ id: 'session-init', name: '会话 1', createdAt: FROZEN_TS }],
    activeSessionId: 'session-init',
    currentSkillId: null,
    skillServiceOffline: false,
  });
});

beforeEach(() => {
  useAppStore.setState({
    ...originalState,
    skillReloadStatus: {},
    wizardMode: { active: false, step: 0, totalSteps: 0 },
    centerView: { kind: 'empty' },
    skills: [],
    sessions: [{ id: 'session-init', name: '会话 1', createdAt: FROZEN_TS }],
    activeSessionId: 'session-init',
    currentSkillId: null,
    skillServiceOffline: false,
  });
});

describe('SkillManagerMenu (Plan 05-07 wizard-only per D5-IA-04 + D5-IA-05)', () => {
  it('renders the trigger button + opens the dropdown on click', () => {
    render(<SkillManagerMenu />);
    expect(screen.queryByTestId('skill-manager-dropdown')).toBeNull();
    fireEvent.click(screen.getByTestId('skill-manager-trigger'));
    expect(screen.getByTestId('skill-manager-dropdown')).toBeInTheDocument();
  });

  it('dropdown contains the 新建 Skill（向导） entry (data-testid=skill-manager-new-skill)', () => {
    render(<SkillManagerMenu />);
    fireEvent.click(screen.getByTestId('skill-manager-trigger'));
    expect(screen.getByTestId('skill-manager-new-skill')).toBeInTheDocument();
    expect(screen.getByText('新建 Skill（向导）')).toBeInTheDocument();
  });

  it('clicking 新建 Skill（向导） sets wizardMode + routes center to skill-md', () => {
    render(<SkillManagerMenu />);
    fireEvent.click(screen.getByTestId('skill-manager-trigger'));
    fireEvent.click(screen.getByTestId('skill-manager-new-skill'));
    const state = useAppStore.getState();
    expect(state.wizardMode.active).toBe(true);
    expect(state.centerView.kind).toBe('skill-md');
  });

  it('dropdown does NOT contain the 全部重载 button (D5-IA-05 — removed)', () => {
    render(<SkillManagerMenu />);
    fireEvent.click(screen.getByTestId('skill-manager-trigger'));
    expect(screen.queryByTestId('skill-manager-reload-all')).toBeNull();
  });

  it('dropdown does NOT contain any skill-manager-item rows (per-skill list moved to LeftSidebar)', () => {
    // Seed the live skills so the menu would have data to render if it
    // still showed the per-skill list. The assertion still must hold.
    useAppStore.setState({
      skills: [
        { id: 's1', name: 'S1', description: 'd', status: 'ok' },
        { id: 's2', name: 'S2', description: 'd', status: 'ok' },
      ],
    });
    render(<SkillManagerMenu />);
    fireEvent.click(screen.getByTestId('skill-manager-trigger'));
    expect(screen.queryAllByTestId('skill-manager-item')).toHaveLength(0);
  });

  it('clicking outside closes the dropdown', () => {
    render(
      <div>
        <div data-testid="outside">outside</div>
        <SkillManagerMenu />
      </div>,
    );
    fireEvent.click(screen.getByTestId('skill-manager-trigger'));
    expect(screen.getByTestId('skill-manager-dropdown')).toBeInTheDocument();
    fireEvent.mouseDown(screen.getByTestId('outside'));
    expect(screen.queryByTestId('skill-manager-dropdown')).toBeNull();
  });
});