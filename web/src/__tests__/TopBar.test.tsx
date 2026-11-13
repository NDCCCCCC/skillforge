// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Open Design redesign B3 — TopBar full-composition tests. The TopBar
// re-homes every deleted-LeftSidebar surface:
//   1. SessionPicker trigger renders (live store data); the SkillPicker
//      is NOT here anymore — 261113-lhu moved it into the composer.
//   2. The 知识库 button opens the KnowledgeModal.
//   3. The Eval button hops to /eval (pushState + route + subroute).
//   4. The detail-panel toggle and ThemeToggle are present.
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';
import TopBar from '../components/layout/TopBar';
import { useAppStore } from '../store/app-store';

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  window.history.pushState({}, '', '/');
  window.localStorage.removeItem('sf.theme');
  document.documentElement.removeAttribute('data-theme');
});

beforeEach(() => {
  // SessionPicker reads the live store slice; feed it a deterministic
  // surface (skills stays seeded — harmless, and the SkillPicker
  // surface is now pinned by SkillPicker.test / ChatInput.test). The
  // knowledge fetch is stubbed away (modal unopened).
  useAppStore.setState({
    skills: [
      { id: 'simple-qa', name: 'simple-qa', description: '简单问答', status: 'ok' },
      { id: 'calculator', name: 'calculator', description: '计算器', status: 'ok' },
    ],
    sessions: [
      { id: 'session-init', name: '会话 1', createdAt: 1 },
      { id: 'session-2', name: '会话 2', createdAt: 2 },
    ],
    activeSessionId: 'session-init',
    sessionPanelOpen: false,
    route: 'chat',
    evalSubroute: 'dashboard',
    skillServiceOffline: false,
  });
  vi.stubGlobal('fetch', vi.fn(async () => ({ ok: true, json: async () => ({ tiers: [] }) } as unknown as Response)));
});

describe('TopBar', () => {
  // 261113-lhu — the Skill 切换入口已下沉 composer（ChatInput 内的
  // SkillPicker）；TopBar 不再承载它，此处钉住"不回流"。
  it('renders the SessionPicker trigger and no longer hosts SkillPicker', () => {
    render(<TopBar />);
    expect(screen.queryByTestId('skill-picker-trigger')).not.toBeInTheDocument();
    expect(screen.getByTestId('session-picker-trigger')).toBeInTheDocument();
    expect(screen.getByTestId('session-picker-trigger').textContent).toContain('会话 1');
  });

  it('opens the KnowledgeModal from the 知识库 button', () => {
    render(<TopBar />);
    expect(screen.queryByTestId('knowledge-modal')).toBeNull();
    fireEvent.click(screen.getByTestId('topbar-knowledge'));
    expect(screen.getByTestId('knowledge-modal')).toBeInTheDocument();
  });

  // Session-management phase — the always-visible panel toggle (the
  // SessionPicker dropdown is hidden below md; this button is the
  // mobile session surface). The panel itself is mounted by
  // ChatLayout; the TopBar owns only the toggle. 261113-jau: the
  // button moved to the LEFT zone (beside SessionPicker) but keeps
  // its testid + toggle behavior, so this test is unchanged.
  it('toggles the session panel flag from the 会话列表 button', () => {
    render(<TopBar />);
    expect(useAppStore.getState().sessionPanelOpen).toBe(false);
    fireEvent.click(screen.getByTestId('topbar-sessions'));
    expect(useAppStore.getState().sessionPanelOpen).toBe(true);
    fireEvent.click(screen.getByTestId('topbar-sessions'));
    expect(useAppStore.getState().sessionPanelOpen).toBe(false);
  });

  // 261113-jau — the right-zone 新建会话 button dispatches the store
  // createSession action: a new roster row is appended and activated.
  // createSession is a pure in-memory mint (no fetch), so state
  // assertions are sufficient. Note: module-level lastMint (double-
  // dispatch guard) can carry across tests in this file, but the guard
  // only fires when activeSessionId equals the last mint — beforeEach
  // resets it to 'session-init', so the mint always proceeds here.
  it('the 新建会话 button dispatches createSession (new roster row + active flip)', () => {
    render(<TopBar />);
    const before = useAppStore.getState().sessions.length;
    fireEvent.click(screen.getByTestId('topbar-new-session'));
    const state = useAppStore.getState();
    expect(state.sessions.length).toBe(before + 1);
    expect(state.activeSessionId).toBe(state.sessions[state.sessions.length - 1].id);
  });

  it('the Eval button hops to /eval with route + dashboard subroute', () => {
    render(<TopBar />);
    fireEvent.click(screen.getByTestId('topbar-eval'));
    expect(window.location.pathname).toBe('/eval');
    expect(useAppStore.getState().route).toBe('eval');
    expect(useAppStore.getState().evalSubroute).toBe('dashboard');
  });

  it('keeps the detail-panel toggle and ThemeToggle', () => {
    render(<TopBar />);
    expect(screen.getByTestId('toggle-detail-panel')).toBeInTheDocument();
    expect(screen.getByTestId('theme-toggle')).toBeInTheDocument();
  });

  it('the ThemeToggle flips the html data-theme attribute', () => {
    render(<TopBar />);
    const toggle = screen.getByTestId('theme-toggle');
    expect(document.documentElement.dataset.theme).toBe('dark');
    fireEvent.click(toggle);
    expect(document.documentElement.dataset.theme).toBe('light');
    expect(window.localStorage.getItem('sf.theme')).toBe('light');
  });
});
