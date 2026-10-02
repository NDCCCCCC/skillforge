// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Open Design redesign B9 — ChatLayout tests. Pins the overlay shell
// contract:
//   1. chat-body mounts the chat column; the detail panel wrap is
//      ALWAYS mounted and flips data-open/inert while closed (CSS
//      slides it off-screen — jsdom has no CSS, so we assert the
//      attributes that drive it).
//   2. Toggling detailPanelOpen flips data-open, drops inert and keeps
//      the store-driven width on the wrap.
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
  it('renders chat-body with the chat column and a closed detail overlay by default', () => {
    render(<ChatLayout chat={<div data-testid="chat-slot" />} detail={<div data-testid="detail-slot" />} />);
    expect(screen.getByTestId('chat-body')).toBeInTheDocument();
    expect(screen.getByTestId('chat-col')).toBeInTheDocument();
    expect(screen.getByTestId('chat-slot')).toBeInTheDocument();
    // B9 — the wrap + handle stay mounted; closed state is expressed
    // via data-open + inert (CSS handles the slide/visibility).
    const wrap = screen.getByTestId('detail-panel-wrap');
    expect(wrap).toHaveAttribute('data-open', 'false');
    expect(wrap).toHaveAttribute('inert');
    expect(screen.getByTestId('dp-resize-handle')).toBeInTheDocument();
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
    const wrap = screen.getByTestId('detail-panel-wrap');
    expect(wrap).toHaveAttribute('data-open', 'false');
    expect(wrap).toHaveAttribute('inert');
  });

  it('flips the detail overlay to open (data-open, no inert) when detailPanelOpen=true', () => {
    useAppStore.setState({ detailPanelOpen: true, detailPanelWidth: 420 });
    render(<ChatLayout chat={<div />} detail={<div data-testid="detail-slot" />} />);
    const wrap = screen.getByTestId('detail-panel-wrap');
    expect(wrap).toBeInTheDocument();
    expect(wrap).toHaveAttribute('data-open', 'true');
    expect(wrap).not.toHaveAttribute('inert');
    expect(screen.getByTestId('detail-slot')).toBeInTheDocument();
    expect(screen.getByTestId('dp-resize-handle')).toBeInTheDocument();
    // Store-driven width lands on the wrapper as an inline style.
    expect(wrap).toHaveStyle({ width: '420px' });
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
  // detail-panel overlay contract: wrap always mounted, data-open +
  // inert while closed, sp-scrim dismiss, sf.sp.* hydration.
  it('flips the session overlay open (data-open, no inert) when sessionPanelOpen=true and the roster renders', () => {
    useAppStore.setState({
      sessionPanelOpen: true,
      sessions: [{ id: 'sess-live-00001', name: '现场会话', createdAt: 1 }],
    });
    render(<ChatLayout chat={<div />} detail={<div />} />);
    const wrap = screen.getByTestId('session-panel-wrap');
    expect(wrap).toHaveAttribute('data-open', 'true');
    expect(wrap).not.toHaveAttribute('inert');
    expect(screen.getByTestId('sp-resize-handle')).toBeInTheDocument();
    expect(screen.getByTestId('session-drawer')).toBeInTheDocument();
    expect(screen.getByText('现场会话')).toBeInTheDocument();
    // The chat column stays mounted alongside — overlay, not replaced.
    expect(screen.getByTestId('chat-col')).toBeInTheDocument();
  });

  it('the sp-scrim closes the overlay (data-open flips to false)', () => {
    useAppStore.setState({ sessionPanelOpen: true });
    render(<ChatLayout chat={<div />} detail={<div />} />);
    fireEvent.click(screen.getByTestId('sp-scrim'));
    expect(useAppStore.getState().sessionPanelOpen).toBe(false);
    const wrap = screen.getByTestId('session-panel-wrap');
    expect(wrap).toHaveAttribute('data-open', 'false');
    expect(wrap).toHaveAttribute('inert');
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
