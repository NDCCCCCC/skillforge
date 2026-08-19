// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 04-03 Task 2 — SkillPicker tests.
//
// Pins the 4 acceptance assertions for the FEUI-04 / SYSK-03 skill
// dropdown:
//   1. Clicking the trigger opens the menu (one item per skill in
//      skills.json).
//   2. Clicking an item sets currentSkillId and closes the menu.
//   3. Clicking an item also dispatches setCenterView({ kind:
//      'skill-md', skillId }) so the center panel swaps.
//   4. The active skill in the menu is highlighted (bg-primary-dim).
//   5. Clicking outside the picker closes the menu.
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import '@testing-library/jest-dom/vitest';
import SkillPicker from '../components/chat/parts/SkillPicker';
import { useAppStore } from '../store/app-store';

afterEach(cleanup);
beforeEach(() => {
  // B3 — the picker reads the LIVE skill surface (useAppStore.skills),
  // so tests seed the store directly (no skills.json fixture import).
  useAppStore.setState({
    currentSkillId: null,
    centerView: { kind: 'empty' },
    skillServiceOffline: false,
    skills: [
      { id: 'simple-qa', name: 'simple-qa', description: '简单问答', status: 'ok' },
      { id: 'calculator', name: 'calculator', description: '计算器', status: 'ok' },
    ],
  });
});

describe('SkillPicker', () => {
  it('clicking the trigger opens the menu with one item per skill', () => {
    render(<SkillPicker />);
    expect(screen.queryByTestId('skill-picker-menu')).toBeNull();
    fireEvent.click(screen.getByTestId('skill-picker-trigger'));
    const menu = screen.getByTestId('skill-picker-menu');
    expect(menu).toBeInTheDocument();
    const items = screen.getAllByTestId('skill-picker-item');
    expect(items.length).toBeGreaterThanOrEqual(2);
  });

  it('clicking an item sets currentSkillId and closes the menu', () => {
    render(<SkillPicker />);
    fireEvent.click(screen.getByTestId('skill-picker-trigger'));
    const items = screen.getAllByTestId('skill-picker-item');
    fireEvent.click(items[0]);
    expect(useAppStore.getState().currentSkillId).toBeTruthy();
    // After selection, the menu closes.
    expect(screen.queryByTestId('skill-picker-menu')).toBeNull();
  });

  it('clicking an item dispatches setCenterView({ kind: skill-md, skillId })', () => {
    render(<SkillPicker />);
    fireEvent.click(screen.getByTestId('skill-picker-trigger'));
    const items = screen.getAllByTestId('skill-picker-item');
    fireEvent.click(items[0]);
    const centerView = useAppStore.getState().centerView;
    expect(centerView.kind).toBe('skill-md');
    if (centerView.kind === 'skill-md') {
      expect(centerView.skillId).toBeTruthy();
    }
  });

  it('the active skill in the menu has the bg-primary-dim class', () => {
    useAppStore.setState({ currentSkillId: 'simple-qa' });
    render(<SkillPicker />);
    fireEvent.click(screen.getByTestId('skill-picker-trigger'));
    const activeItem = screen.getByTestId('skill-picker-item-active');
    expect(activeItem).toHaveClass('bg-primary-dim');
    expect(activeItem).toHaveAttribute('data-skill-id', 'simple-qa');
  });

  it('clicking outside the picker closes the menu', () => {
    render(
      <div>
        <div data-testid="outside">outside</div>
        <SkillPicker />
      </div>,
    );
    fireEvent.click(screen.getByTestId('skill-picker-trigger'));
    expect(screen.getByTestId('skill-picker-menu')).toBeInTheDocument();
    fireEvent.mouseDown(screen.getByTestId('outside'));
    expect(screen.queryByTestId('skill-picker-menu')).toBeNull();
  });
});
