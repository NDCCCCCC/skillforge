// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 05-03 Task 2 — SkillManagerMenu tests.
//
// Pins the SC-2 + SC-3 acceptance assertions for the topbar dropdown:
//   1. Trigger button renders + toggles the dropdown.
//   2. Dropdown lists every installed Skill with per-skill status.
//   3. Per-skill status shows 已加载 when skillReloadStatus is empty.
//   4. 全部重载 CTA dispatches a POST to the reload path.
//   5. 新建 Skill（向导） CTA sets wizardMode + routes center to skill-md.
//   6. Per-skill status shows 加载失败（旧版可用） when reload status
//      has a 'failed' entry for that skill.
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';
import SkillManagerMenu from '../components/layout/SkillManagerMenu';
import { useAppStore } from '../store/app-store';

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  // Reset store mutations from the test
  useAppStore.setState({
    skillReloadStatus: {},
    wizardMode: { active: false, step: 0, totalSteps: 0 },
    centerView: { kind: 'empty' },
  });
});

beforeEach(() => {
  useAppStore.setState({
    skillReloadStatus: {},
    wizardMode: { active: false, step: 0, totalSteps: 0 },
    centerView: { kind: 'empty' },
  });
});

describe('SkillManagerMenu', () => {
  it('renders the trigger button + opens the dropdown on click', () => {
    render(<SkillManagerMenu />);
    expect(screen.queryByTestId('skill-manager-dropdown')).toBeNull();
    fireEvent.click(screen.getByTestId('skill-manager-trigger'));
    expect(screen.getByTestId('skill-manager-dropdown')).toBeInTheDocument();
  });

  it('lists every installed skill with per-skill status', () => {
    render(<SkillManagerMenu />);
    fireEvent.click(screen.getByTestId('skill-manager-trigger'));
    const items = screen.getAllByTestId('skill-manager-item');
    expect(items.length).toBeGreaterThanOrEqual(2);
    // Default status when skillReloadStatus is empty: 已加载
    expect(within(items[0]).getByText('已加载')).toBeInTheDocument();
  });

  it('shows 加载失败（旧版可用） when reload status is failed for a skill', () => {
    useAppStore.setState({
      skillReloadStatus: {
        'compliance-review': {
          status: 'failed',
          version: 'old-kept',
          error: '缺字段 temperature',
          ts: Date.now(),
          skillName: '企业合规/制度审查',
        },
      },
    });
    render(<SkillManagerMenu />);
    fireEvent.click(screen.getByTestId('skill-manager-trigger'));
    const items = screen.getAllByTestId('skill-manager-item');
    const failedItem = items.find(
      (el) => el.getAttribute('data-skill-id') === 'compliance-review',
    ) as HTMLElement | undefined;
    expect(failedItem).toBeDefined();
    if (failedItem) {
      expect(within(failedItem).getByText('加载失败（旧版可用）')).toBeInTheDocument();
      expect(failedItem.getAttribute('title')).toContain('缺字段 temperature');
    }
  });

  it('clicking 全部重载 dispatches a POST to the reload path', () => {
    const fetchMock = vi.fn().mockResolvedValue({ ok: true } as unknown as Response);
    vi.stubGlobal('fetch', fetchMock);
    render(<SkillManagerMenu reloadPath="/api/test/reload" />);
    fireEvent.click(screen.getByTestId('skill-manager-trigger'));
    fireEvent.click(screen.getByTestId('skill-manager-reload-all'));
    expect(fetchMock).toHaveBeenCalledWith('/api/test/reload', expect.objectContaining({ method: 'POST' }));
  });

  it('clicking 新建 Skill（向导） sets wizardMode + routes center to skill-md', () => {
    render(<SkillManagerMenu />);
    fireEvent.click(screen.getByTestId('skill-manager-trigger'));
    fireEvent.click(screen.getByTestId('skill-manager-new-skill'));
    const state = useAppStore.getState();
    expect(state.wizardMode.active).toBe(true);
    expect(state.centerView.kind).toBe('skill-md');
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
