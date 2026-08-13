// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 05-07 Task 3 + Plan 05-08 Task 3 — LeftSidebar tests.
//
// LeftSidebar renders 4 stacked labeled sections (D5-IA-02/03/04):
//   1. 新建会话 — button that calls createSession().
//   2. 会话 — list of store.sessions (UI-local, D5-IA-03). Each row
//      ALSO exposes a × delete button (IN-01 closure) that calls
//      deleteSession with stopPropagation so the row's setActiveSession
//      does not double-fire.
//   3. 文件 — preserved 4-tier SOURCES list (mocked; real file tree
//      is a separate effort per CONTEXT deferred items).
//   4. 技能 — list of store.skills with status badges
//      (已加载 / 加载失败（旧版可用） per UI-SPEC §Copywriting
//      Contract Phase-4-deferred items). Failed-skill title attribute
//      is passed through stripFsPath + multi-line flattening (WR-01/04).
//      Loud-failure banner (IN-04) renders above when
//      skillServiceOffline=true.
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import '@testing-library/jest-dom/vitest';
import LeftSidebar from '../components/layout/LeftSidebar';
import { useAppStore } from '../store/app-store';

const FROZEN_TS = 1700000000000;
const originalState = useAppStore.getState();

afterEach(() => {
  cleanup();
  // Reset every slice this test file touches.
  useAppStore.setState({
    ...originalState,
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
    skills: [],
    sessions: [{ id: 'session-init', name: '会话 1', createdAt: FROZEN_TS }],
    activeSessionId: 'session-init',
    currentSkillId: null,
    skillServiceOffline: false,
  });
});

describe('LeftSidebar (Plan 05-07 D5-IA-02/03/04 + Plan 05-08 WR-01/04/IN-01/IN-04)', () => {
  it('renders the 新建会话 button (data-testid=left-sidebar-new-session)', () => {
    render(<LeftSidebar />);
    const btn = screen.getByTestId('left-sidebar-new-session');
    expect(btn).toBeInTheDocument();
  });

  it('clicking 新建会话 calls createSession — sessions grows by 1 and the new session is active', () => {
    render(<LeftSidebar />);
    const initialCount = useAppStore.getState().sessions.length;
    const initialActive = useAppStore.getState().activeSessionId;
    fireEvent.click(screen.getByTestId('left-sidebar-new-session'));
    const after = useAppStore.getState();
    expect(after.sessions.length).toBe(initialCount + 1);
    expect(after.activeSessionId).not.toBe(initialActive);
    expect(after.sessions[after.sessions.length - 1].id).toBe(after.activeSessionId);
  });

  it('seeds two sessions with activeSessionId=s1 — renders two left-sidebar-session rows; s1 is the active row', () => {
    useAppStore.setState({
      sessions: [
        { id: 's1', name: '会话 1', createdAt: FROZEN_TS },
        { id: 's2', name: '会话 2', createdAt: FROZEN_TS + 1 },
      ],
      activeSessionId: 's1',
    });
    render(<LeftSidebar />);
    const rows = screen.getAllByTestId('left-sidebar-session');
    expect(rows).toHaveLength(2);
    const s1Row = rows.find((r) => r.getAttribute('data-session-id') === 's1') as HTMLElement | undefined;
    expect(s1Row).toBeDefined();
    if (s1Row) {
      // Active row carries border-l-primary + bg-primary-dim.
      const cls = s1Row.className;
      expect(cls).toMatch(/border-l-primary|bg-primary-dim/);
    }
  });

  it('clicking the s2 row calls setActiveSession — activeSessionId becomes s2', () => {
    useAppStore.setState({
      sessions: [
        { id: 's1', name: '会话 1', createdAt: FROZEN_TS },
        { id: 's2', name: '会话 2', createdAt: FROZEN_TS + 1 },
      ],
      activeSessionId: 's1',
    });
    render(<LeftSidebar />);
    const rows = screen.getAllByTestId('left-sidebar-session');
    const s2Row = rows.find((r) => r.getAttribute('data-session-id') === 's2') as HTMLElement | undefined;
    expect(s2Row).toBeDefined();
    if (s2Row) fireEvent.click(s2Row);
    expect(useAppStore.getState().activeSessionId).toBe('s2');
  });

  it('seeds skills with one ok skill — renders a left-sidebar-skill row with name + 已加载, marked active', () => {
    useAppStore.setState({
      skills: [
        {
          id: 'compliance-review',
          name: '企业合规/制度审查',
          description: 'desc',
          status: 'ok',
        },
      ],
      currentSkillId: 'compliance-review',
    });
    render(<LeftSidebar />);
    const rows = screen.getAllByTestId('left-sidebar-skill');
    expect(rows).toHaveLength(1);
    const row = rows[0];
    expect(within(row).getByText('企业合规/制度审查')).toBeInTheDocument();
    expect(within(row).getByText('已加载')).toBeInTheDocument();
    expect(row.className).toMatch(/border-l-primary/);
  });

  it('seeds a failed skill — renders 加载失败（旧版可用） with title containing the error', () => {
    useAppStore.setState({
      skills: [
        {
          id: 'broken',
          name: 'broken-skill',
          description: 'd',
          status: 'failed',
          error: '缺字段 temperature',
        },
      ],
    });
    render(<LeftSidebar />);
    const rows = screen.getAllByTestId('left-sidebar-skill');
    expect(rows).toHaveLength(1);
    const row = rows[0];
    expect(within(row).getByText('加载失败（旧版可用）')).toBeInTheDocument();
    expect(row.getAttribute('title') ?? '').toContain('缺字段 temperature');
  });

  it('empty skills renders 尚未安装任何 Skill with no crash', () => {
    render(<LeftSidebar />);
    expect(screen.getByText('尚未安装任何 Skill')).toBeInTheDocument();
  });

  it('文件 block still renders the 4 SOURCES tiers (权威/历史/个人/草稿)', () => {
    render(<LeftSidebar />);
    expect(screen.getByText('权威')).toBeInTheDocument();
    expect(screen.getByText('历史')).toBeInTheDocument();
    expect(screen.getByText('个人')).toBeInTheDocument();
    expect(screen.getByText('草稿')).toBeInTheDocument();
  });

  it('Plan 05-08 IN-01 — renders a × delete button per session row', () => {
    useAppStore.setState({
      sessions: [
        { id: 's1', name: '会话 1', createdAt: FROZEN_TS },
        { id: 's2', name: '会话 2', createdAt: FROZEN_TS + 1 },
      ],
      activeSessionId: 's1',
    });
    render(<LeftSidebar />);
    const deleteBtns = screen.getAllByTestId('left-sidebar-session-delete');
    expect(deleteBtns).toHaveLength(2);
    // Click × on s2 (the inactive one) — only s2 should be removed
    // and activeSessionId should stay on s1.
    const s2Delete = deleteBtns.find(
      (b) => b.getAttribute('data-session-id') === 's2',
    ) as HTMLElement | undefined;
    expect(s2Delete).toBeDefined();
    if (s2Delete) fireEvent.click(s2Delete);
    expect(useAppStore.getState().sessions.length).toBe(1);
    expect(useAppStore.getState().activeSessionId).toBe('s1');
  });

  it('Plan 05-08 IN-04 — renders a skill-service-offline banner when skillServiceOffline is true', () => {
    useAppStore.setState({ skillServiceOffline: true });
    render(<LeftSidebar />);
    const banner = screen.getByTestId('skill-service-offline-banner');
    expect(banner).toBeInTheDocument();
    expect(banner.textContent).toContain('无法连接到 Skill 服务');
  });

  it('Plan 05-08 WR-01 + WR-04 — strips FS paths and flattens newlines in the failed-skill title', () => {
    useAppStore.setState({
      skills: [
        {
          id: 'broken',
          name: 'broken-skill',
          description: 'd',
          status: 'failed',
          error: '/var/lib/secrets/x: missing temperature\n  at parse\n  at validate',
        },
      ],
    });
    render(<LeftSidebar />);
    const row = screen.getAllByTestId('left-sidebar-skill')[0];
    const title = row.getAttribute('title') ?? '';
    // Path stripped to basename; newlines flattened to spaces
    // (whitespace is not collapsed — just \n / \r\n → ' ' for OS-stable
    // tooltip rendering). Multi-space runs may survive the flatten.
    const titleCompressed = title.replace(/\s+/g, ' ');
    expect(titleCompressed).toContain('x: missing temperature at parse');
    expect(title).not.toContain('/var/lib/secrets');
    expect(title).not.toContain('\n');
  });
});