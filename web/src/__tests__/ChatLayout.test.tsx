// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Open Design redesign B2 — ChatLayout tests. Pins the chat-first
// shell contract:
//   1. chat-body mounts the chat column; the detail panel is hidden
//      while detailPanelOpen=false (default).
//   2. Toggling detailPanelOpen mounts the resize handle + the detail
//      panel wrap with the store-driven width.
//   3. The width clamps to 320–520 via localStorage hydration
//      (sf.dp.w out-of-range values are clamped).
//   4. The stored open flag (sf.dp.open='1') re-opens the panel on
//      mount.
import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import '@testing-library/jest-dom/vitest';
import ChatLayout from '../components/layout/ChatLayout';
import { useAppStore } from '../store/app-store';

afterEach(() => {
  cleanup();
  window.localStorage.clear();
  useAppStore.setState({ detailPanelOpen: false, detailPanelWidth: 380 });
});

beforeEach(() => {
  window.localStorage.clear();
  useAppStore.setState({ detailPanelOpen: false, detailPanelWidth: 380 });
});

describe('ChatLayout (Open Design redesign B2)', () => {
  it('renders chat-body with the chat column and no detail panel by default', () => {
    render(<ChatLayout chat={<div data-testid="chat-slot" />} detail={<div data-testid="detail-slot" />} />);
    expect(screen.getByTestId('chat-body')).toBeInTheDocument();
    expect(screen.getByTestId('chat-col')).toBeInTheDocument();
    expect(screen.getByTestId('chat-slot')).toBeInTheDocument();
    expect(screen.queryByTestId('detail-panel-wrap')).toBeNull();
    expect(screen.queryByTestId('dp-resize-handle')).toBeNull();
  });

  it('mounts the detail panel + resize handle when detailPanelOpen=true', () => {
    useAppStore.setState({ detailPanelOpen: true, detailPanelWidth: 420 });
    render(<ChatLayout chat={<div />} detail={<div data-testid="detail-slot" />} />);
    expect(screen.getByTestId('detail-panel-wrap')).toBeInTheDocument();
    expect(screen.getByTestId('detail-slot')).toBeInTheDocument();
    expect(screen.getByTestId('dp-resize-handle')).toBeInTheDocument();
    // Store-driven width lands on the wrapper as an inline style.
    expect(screen.getByTestId('detail-panel-wrap')).toHaveStyle({ width: '420px' });
  });

  it('clamps an out-of-range stored width (sf.dp.w) into 320–520 on hydration', () => {
    window.localStorage.setItem('sf.dp.w', '9999');
    render(<ChatLayout chat={<div />} detail={<div />} />);
    expect(useAppStore.getState().detailPanelWidth).toBe(520);
  });

  it('re-opens the panel when sf.dp.open === "1"', () => {
    window.localStorage.setItem('sf.dp.open', '1');
    render(<ChatLayout chat={<div />} detail={<div data-testid="detail-slot" />} />);
    expect(useAppStore.getState().detailPanelOpen).toBe(true);
    expect(screen.getByTestId('detail-slot')).toBeInTheDocument();
  });

  it('keeps the shell grid: topbar + chat-body + statusbar', () => {
    render(<ChatLayout chat={<div />} detail={<div />} />);
    expect(screen.getByTestId('app-topbar')).toBeInTheDocument();
    expect(screen.getByTestId('app-statusbar')).toBeInTheDocument();
    expect(screen.getByTestId('chat-body')).toBeInTheDocument();
  });
});
