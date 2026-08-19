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
//   5. B7 — the <1280px overlay scrim (dp-scrim) mounts only while the
//      panel is open and clicking it closes the panel.
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import '@testing-library/jest-dom/vitest';
import ChatLayout from '../components/layout/ChatLayout';
import { useAppStore } from '../store/app-store';

afterEach(() => {
  cleanup();
  window.localStorage.clear();
  useAppStore.setState({
    detailPanelOpen: false,
    detailPanelWidth: 380,
    sessionPanelOpen: false,
    sessionPanelWidth: 280,
  });
});

beforeEach(() => {
  window.localStorage.clear();
  useAppStore.setState({
    detailPanelOpen: false,
    detailPanelWidth: 380,
    sessionPanelOpen: false,
    sessionPanelWidth: 280,
  });
});

describe('ChatLayout (Open Design redesign B2)', () => {
  it('renders chat-body with the chat column and no detail panel by default', () => {
    render(<ChatLayout chat={<div data-testid="chat-slot" />} detail={<div data-testid="detail-slot" />} />);
    expect(screen.getByTestId('chat-body')).toBeInTheDocument();
    expect(screen.getByTestId('chat-col')).toBeInTheDocument();
    expect(screen.getByTestId('chat-slot')).toBeInTheDocument();
    expect(screen.queryByTestId('detail-panel-wrap')).toBeNull();
    expect(screen.queryByTestId('dp-resize-handle')).toBeNull();
    // B7 — the mobile-overlay scrim is NOT mounted while closed either.
    expect(screen.queryByTestId('dp-scrim')).toBeNull();
  });

  it('mounts the <1280px overlay scrim while open and closes the panel on click (B7)', () => {
    useAppStore.setState({ detailPanelOpen: true });
    render(<ChatLayout chat={<div />} detail={<div data-testid="detail-slot" />} />);
    const scrim = screen.getByTestId('dp-scrim');
    expect(scrim).toBeInTheDocument();
    fireEvent.click(scrim);
    expect(useAppStore.getState().detailPanelOpen).toBe(false);
    expect(screen.queryByTestId('detail-panel-wrap')).toBeNull();
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

  // Session-management phase — the LEFT session panel mirrors the
  // detail-panel contract: inline wrap + resize handle while open,
  // none while closed, sp-scrim dismiss, sf.sp.* hydration.
  it('mounts the session panel + handle when sessionPanelOpen=true and the roster renders', () => {
    useAppStore.setState({
      sessionPanelOpen: true,
      sessions: [{ id: 'sess-live-00001', name: '现场会话', createdAt: 1 }],
    });
    render(<ChatLayout chat={<div />} detail={<div />} />);
    expect(screen.getByTestId('session-panel-wrap')).toBeInTheDocument();
    expect(screen.getByTestId('sp-resize-handle')).toBeInTheDocument();
    expect(screen.getByTestId('session-drawer')).toBeInTheDocument();
    expect(screen.getByText('现场会话')).toBeInTheDocument();
    // The chat column stays mounted alongside — inline, not replaced.
    expect(screen.getByTestId('chat-col')).toBeInTheDocument();
  });

  it('hides the session panel + scrim by default and the sp-scrim closes on click', () => {
    useAppStore.setState({ sessionPanelOpen: true });
    render(<ChatLayout chat={<div />} detail={<div />} />);
    fireEvent.click(screen.getByTestId('sp-scrim'));
    expect(useAppStore.getState().sessionPanelOpen).toBe(false);
    expect(screen.queryByTestId('session-panel-wrap')).toBeNull();
  });

  it('clamps an out-of-range stored session width (sf.sp.w) into 220–360 on hydration', () => {
    window.localStorage.setItem('sf.sp.w', '999');
    render(<ChatLayout chat={<div />} detail={<div />} />);
    // Hydration clamps the STORE value; the LS write-back only happens
    // on a drag mouseup (same contract as sf.dp.w).
    expect(useAppStore.getState().sessionPanelWidth).toBe(360);
  });

  it('re-opens the session panel when sf.sp.open === "1"', () => {
    window.localStorage.setItem('sf.sp.open', '1');
    render(<ChatLayout chat={<div />} detail={<div />} />);
    expect(useAppStore.getState().sessionPanelOpen).toBe(true);
    expect(screen.getByTestId('session-panel-wrap')).toBeInTheDocument();
  });
});
