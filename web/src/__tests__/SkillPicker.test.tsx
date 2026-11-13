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
//
// 261113-nkf — placement regression: the picker lives in the composer
// header (above the input box), so when the viewport space below the
// trigger cannot fit the menu the placement flips upward
// (data-placement="top" + bottom positioning class). jsdom returns
// all-zero rects and innerHeight=768 by default, so the flip test
// stubs window.innerHeight and Element.prototype.getBoundingClientRect.
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';
import SkillPicker from '../components/chat/parts/SkillPicker';
import { useAppStore } from '../store/app-store';

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});
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
    // 261113-nkf — default placement is downward (jsdom rects are all
    // zero, so the space below is the whole viewport).
    expect(menu).toHaveAttribute('data-placement', 'bottom');
    expect(menu).toHaveClass('mt-1');
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

  it('flips the menu above the trigger when space below is insufficient', () => {
    // 261113-nkf — composer-header placement. Stub the geometry so the
    // trigger sits 20px above a 200px viewport: spaceBelow = 20 < the
    // estimated menu height (2 skills ≈ min(2*34+40, 320) = 108), and
    // spaceAbove = 160 > 20 → placement flips to top.
    const innerHeightSpy = vi.spyOn(window, 'innerHeight', 'get').mockReturnValue(200);
    const rectSpy = vi
      .spyOn(Element.prototype, 'getBoundingClientRect')
      .mockReturnValue({
        x: 0,
        y: 160,
        top: 160,
        bottom: 180,
        left: 0,
        right: 100,
        width: 100,
        height: 20,
        toJSON: () => ({}),
      } as unknown as DOMRect);

    try {
      render(<SkillPicker />);
      fireEvent.click(screen.getByTestId('skill-picker-trigger'));
      const menu = screen.getByTestId('skill-picker-menu');
      expect(menu).toHaveAttribute('data-placement', 'top');
      expect(menu).toHaveClass('bottom-[calc(100%+4px)]');
      expect(menu).not.toHaveClass('mt-1');
    } finally {
      innerHeightSpy.mockRestore();
      rectSpy.mockRestore();
    }
  });

  it('falls back to downward placement after the flipped menu closes and reopens', () => {
    // 261113-nkf — closing must reset dropUp so a later open with
    // enough room below renders data-placement="bottom" again.
    const innerHeightSpy = vi.spyOn(window, 'innerHeight', 'get').mockReturnValue(200);
    const rectSpy = vi
      .spyOn(Element.prototype, 'getBoundingClientRect')
      .mockImplementation(function (this: Element) {
        if (this.getAttribute('data-testid') === 'skill-picker-trigger') {
          return {
            x: 0,
            y: 160,
            top: 160,
            bottom: 180,
            left: 0,
            right: 100,
            width: 100,
            height: 20,
            toJSON: () => ({}),
          } as unknown as DOMRect;
        }
        return {
          x: 0,
          y: 0,
          top: 0,
          bottom: 0,
          left: 0,
          right: 0,
          width: 0,
          height: 0,
          toJSON: () => ({}),
        } as unknown as DOMRect;
      });

    try {
      render(<SkillPicker />);
      // Open 1: trigger near the viewport bottom → flips to top.
      const trigger = screen.getByTestId('skill-picker-trigger');
      fireEvent.click(trigger);
      expect(screen.getByTestId('skill-picker-menu')).toHaveAttribute('data-placement', 'top');
      // Close (second toggle — document.body is an ancestor of the
      // picker, so the outside-click path would NOT close it).
      fireEvent.click(trigger);
      expect(screen.queryByTestId('skill-picker-menu')).toBeNull();
      // Open 2: trigger moved to the top (plenty of room below).
      trigger.getBoundingClientRect = () => ({
        x: 0,
        y: 10,
        top: 10,
        bottom: 30,
        left: 0,
        right: 100,
        width: 100,
        height: 20,
        toJSON: () => ({}),
      } as unknown as DOMRect);
      fireEvent.click(trigger);
      expect(screen.getByTestId('skill-picker-menu')).toHaveAttribute('data-placement', 'bottom');
    } finally {
      innerHeightSpy.mockRestore();
      rectSpy.mockRestore();
    }
  });
});
