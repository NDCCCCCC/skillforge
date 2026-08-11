// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 04-03 Task 2 — ThreePanel tests.
//
// Pins the 5 acceptance assertions for the draggable three-panel
// layout:
//   1. data-testid="app-main" mounts panel-left, resize-handle-left,
//      panel-center, resize-handle-right, panel-right (3 panels + 2
//      handles = 5 grid tracks)
//   2. Clicking collapse-left sets left column to 0px
//   3. Clicking collapse-right sets right column to 0px
//   4. Default gridTemplateColumns starts with `260px` (leftWidth default)
//   5. localStorage round-trip: drag right handle → localStorage holds new width
//   6. Hydration: pre-populating localStorage for left key drives the
//      rendered gridTemplateColumns.
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import '@testing-library/jest-dom/vitest';
import ThreePanel from '../components/layout/ThreePanel';
import { useAppStore } from '../store/app-store';

afterEach(cleanup);
beforeEach(() => {
  window.localStorage.clear();
  // Reset store to defaults.
  useAppStore.setState({
    leftWidth: 260,
    rightWidth: 420,
    sidebarLeft: true,
    sidebarRight: true,
    centerView: { kind: 'empty' },
    currentSkillId: null,
  });
});

describe('ThreePanel', () => {
  it('renders app-main with three children: panel-left, panel-center, panel-right', () => {
    render(<ThreePanel left={<div>left content</div>} center={<div>center</div>} right={<div>right</div>} />);
    const main = screen.getByTestId('app-main');
    expect(main).toBeInTheDocument();
    expect(screen.getByTestId('panel-left')).toBeInTheDocument();
    expect(screen.getByTestId('panel-center')).toBeInTheDocument();
    expect(screen.getByTestId('panel-right')).toBeInTheDocument();
  });

  it('clicking collapse-left toggles the left panel width to 0', () => {
    render(<ThreePanel left={<div>L</div>} center={<div>C</div>} right={<div>R</div>} />);
    const main = screen.getByTestId('app-main');
    // Default state: left column is `260px`.
    expect(main.style.gridTemplateColumns).toMatch(/^260px/);

    fireEvent.click(screen.getByTestId('collapse-left'));
    expect(useAppStore.getState().sidebarLeft).toBe(false);
    expect(main.style.gridTemplateColumns).toMatch(/^0px/);
  });

  it('clicking collapse-right toggles the right panel width to 0', () => {
    render(<ThreePanel left={<div>L</div>} center={<div>C</div>} right={<div>R</div>} />);
    fireEvent.click(screen.getByTestId('collapse-right'));
    expect(useAppStore.getState().sidebarRight).toBe(false);
    const main = screen.getByTestId('app-main');
    // 5-track grid: [left, handle, center, handle, right]. When right
    // is collapsed, the right column (cols[4]) is `0px`.
    const cols = main.style.gridTemplateColumns.split(' ');
    expect(cols[4]).toBe('0px');
  });

  it('default gridTemplateColumns starts with 260px', () => {
    render(<ThreePanel left={<div>L</div>} center={<div>C</div>} right={<div>R</div>} />);
    const main = screen.getByTestId('app-main');
    expect(main.style.gridTemplateColumns).toMatch(/^260px/);
  });

  it('drag on the right resize handle persists the new width to localStorage', () => {
    render(<ThreePanel left={<div>L</div>} center={<div>C</div>} right={<div>R</div>} />);
    const handle = screen.getByTestId('resize-handle-right');

    // Simulate mousedown → mousemove → mouseup on the right handle.
    fireEvent.mouseDown(handle, { clientX: 800, clientY: 100 });
    // Move cursor left by 200px to shrink the right panel.
    fireEvent.mouseMove(document, { clientX: 600, clientY: 100 });
    fireEvent.mouseUp(document, { clientX: 600, clientY: 100 });

    // After drag-end, localStorage should have the new width recorded.
    const stored = window.localStorage.getItem('sf-right-width');
    expect(stored).toBeTruthy();
    const storedNum = parseInt(stored ?? '0', 10);
    expect(storedNum).toBeGreaterThanOrEqual(180);
    expect(storedNum).toBeLessThanOrEqual(600);
  });

  it('hydrates leftWidth from localStorage on mount', () => {
    window.localStorage.setItem('sf-left-width', '300');
    render(<ThreePanel left={<div>L</div>} center={<div>C</div>} right={<div>R</div>} />);
    const main = screen.getByTestId('app-main');
    // After hydration, the store picks up 300 and the inline grid template
    // starts with 300px.
    expect(main.style.gridTemplateColumns).toMatch(/^300px/);
  });
});
