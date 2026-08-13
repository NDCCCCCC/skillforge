// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 05-07 Task 2 + Plan 05-08 Task 1 — ChatInput skill-selector tests.
//
// Pins D5-IA-01: the chat-input-current-skill block is a clickable Skill
// selector popover listing skills from useAppStore.skills (which is
// populated by useSkillReload reading item.manifest.description).
// Selecting a row calls useAppStore.setCurrentSkill(id). When the skills
// list is empty the selector shows a muted 尚未安装任何 Skill row.
//
// Plan 05-08 (WR-02) — when the selected skill is removed from
// store.skills (e.g. backend dropped the YAML), currentSkillId is
// automatically cleared so the trigger shows '未选择 Skill' and no
// dangling id leaks to /api/chat.
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import '@testing-library/jest-dom/vitest';
import ChatInput from '../components/chat/ChatInput';
import { useAppStore } from '../store/app-store';

const FROZEN_TS = 1700000000000;
const originalState = useAppStore.getState();

afterEach(() => {
  cleanup();
  // Reset every slice this test file touches (sessions/activeSessionId
  // could be touched via setCurrentSkill which lives on the same store).
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

describe('ChatInput Skill selector (Plan 05-07 D5-IA-01 + Plan 05-08 WR-02)', () => {
  it('renders the chat-input-current-skill trigger with 未选择 Skill when no skill is selected', () => {
    render(<ChatInput onSend={() => undefined} />);
    const trigger = screen.getByTestId('chat-input-current-skill');
    expect(trigger).toBeInTheDocument();
    expect(trigger.textContent).toContain('未选择 Skill');
  });

  it('clicking the trigger opens a popover listing skills from store.skills with name + description', () => {
    useAppStore.setState({
      skills: [
        {
          id: 's1',
          name: 'S1',
          description: 'desc-from-manifest',
          status: 'ok',
        },
        {
          id: 's2',
          name: 'S2',
          description: 'second skill',
          status: 'ok',
        },
      ],
    });
    render(<ChatInput onSend={() => undefined} />);
    fireEvent.click(screen.getByTestId('chat-input-current-skill'));
    const rows = screen.getAllByTestId('chat-input-skill-option');
    expect(rows).toHaveLength(2);
    expect(within(rows[0]).getByText('S1')).toBeInTheDocument();
    expect(within(rows[0]).getByText('desc-from-manifest')).toBeInTheDocument();
    expect(within(rows[1]).getByText('S2')).toBeInTheDocument();
    expect(within(rows[1]).getByText('second skill')).toBeInTheDocument();
  });

  it('clicking a skill option sets currentSkillId via the store and closes the popover', () => {
    useAppStore.setState({
      skills: [
        { id: 's1', name: 'S1', description: 'desc', status: 'ok' },
      ],
    });
    render(<ChatInput onSend={() => undefined} />);
    fireEvent.click(screen.getByTestId('chat-input-current-skill'));
    const option = screen.getByTestId('chat-input-skill-option');
    fireEvent.click(option);
    expect(useAppStore.getState().currentSkillId).toBe('s1');
    // Popover closes — queryByTestId returns null when not in DOM.
    expect(screen.queryByTestId('chat-input-skill-option')).toBeNull();
  });

  it('renders a muted 尚未安装任何 Skill row when store.skills is empty (no crash)', () => {
    render(<ChatInput onSend={() => undefined} />);
    fireEvent.click(screen.getByTestId('chat-input-current-skill'));
    expect(screen.queryByTestId('chat-input-skill-option')).toBeNull();
    expect(screen.getByText('尚未安装任何 Skill')).toBeInTheDocument();
  });

  it('Plan 05-08 WR-02 — clears currentSkillId when the selected skill disappears from store.skills', async () => {
    // Seed currentSkillId='s1' BEFORE render so the ChatInput mounts
    // with an active selection.
    useAppStore.setState({
      skills: [{ id: 's1', name: 'S1', description: 'd', status: 'ok' }],
      currentSkillId: 's1',
    });
    render(<ChatInput onSend={() => undefined} />);
    // Trigger shows the active Skill.
    expect(screen.getByTestId('chat-input-current-skill').textContent).toContain('S1');

    // Backend removes the Skill (reload-failed YAML, manual delete, etc.).
    useAppStore.setState({ skills: [] });

    // Wait for the useEffect to clear the stale id.
    await waitFor(() => {
      expect(useAppStore.getState().currentSkillId).toBeNull();
    });
    // Trigger reverts to '未选择 Skill' fallback.
    expect(screen.getByTestId('chat-input-current-skill').textContent).toContain('未选择 Skill');
  });
});