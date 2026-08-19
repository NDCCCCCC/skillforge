// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Open Design redesign B3 — TopBar full-composition tests. The TopBar
// re-homes every deleted-LeftSidebar surface:
//   1. SkillPicker + SessionPicker triggers render (live store data).
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
  // Both pickers read live store slices; feed them a deterministic
  // surface. The knowledge fetch is stubbed away (modal unopened).
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
    route: 'chat',
    evalSubroute: 'dashboard',
    skillServiceOffline: false,
  });
  vi.stubGlobal('fetch', vi.fn(async () => ({ ok: true, json: async () => ({ tiers: [] }) } as unknown as Response)));
});

describe('TopBar', () => {
  it('renders the SkillPicker and SessionPicker triggers', () => {
    render(<TopBar />);
    expect(screen.getByTestId('skill-picker-trigger')).toBeInTheDocument();
    expect(screen.getByTestId('session-picker-trigger')).toBeInTheDocument();
    expect(screen.getByTestId('session-picker-trigger').textContent).toContain('会话 1');
  });

  it('opens the KnowledgeModal from the 知识库 button', () => {
    render(<TopBar />);
    expect(screen.queryByTestId('knowledge-modal')).toBeNull();
    fireEvent.click(screen.getByTestId('topbar-knowledge'));
    expect(screen.getByTestId('knowledge-modal')).toBeInTheDocument();
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
