// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Quick plan 260815-0xb Task 2 — ChatInput slash-command Skill menu
// tests (Claude Code-style interaction, replacing the Plan 05-07
// D5-IA-01 click-popover tests).
//
// Pins the slash-menu contract:
//  - typing "/" (non-wizard mode) opens chat-input-skill-menu listing
//    store.skills name + description
//  - continued typing filters (case-insensitive substring on name +
//    description)
//  - ArrowDown/ArrowUp move the data-active highlight (clamped, cyclic);
//    Enter selects (setCurrentSkill + clear input + close menu) and does
//    NOT send
//  - Esc closes the menu keeping the typed text; outside mousedown also
//    closes
//  - empty skills renders the 尚未安装任何 Skill empty row (no crash)
//  - wizard mode (modeLabel) never opens the menu; Plan 05-08 WR-02
//    stale-skill defense still clears a dangling currentSkillId
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
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

function seedSkills() {
  useAppStore.setState({
    skills: [
      { id: 'calculator', name: 'Calculator', description: 'performs arithmetic', status: 'ok' },
      { id: 'compliance', name: 'Compliance Review', description: '合规审查 skill', status: 'ok' },
    ],
  });
}

function type(value: string) {
  fireEvent.change(screen.getByTestId('chat-input-textarea'), { target: { value } });
}

describe('ChatInput slash-command Skill menu (quick plan 260815-0xb)', () => {
  it('renders the read-only current-skill badge; typing "/" opens the menu listing skills', () => {
    seedSkills();
    render(<ChatInput onSend={() => undefined} />);
    // Read-only badge — a div, not a clickable popover trigger.
    const badge = screen.getByTestId('chat-input-current-skill');
    expect(badge.textContent).toContain('未选择 Skill');
    // Menu closed before any input.
    expect(screen.queryByTestId('chat-input-skill-menu')).toBeNull();
    type('/');
    const menu = screen.getByTestId('chat-input-skill-menu');
    expect(within_text(menu, 'Calculator')).toBe(true);
    expect(within_text(menu, 'performs arithmetic')).toBe(true);
    expect(within_text(menu, 'Compliance Review')).toBe(true);
  });

  it('continued typing filters skills by case-insensitive substring on name or description', () => {
    seedSkills();
    render(<ChatInput onSend={() => undefined} />);
    type('/cal');
    const menu = screen.getByTestId('chat-input-skill-menu');
    // "Calculator" name matches; "Compliance Review"/"合规审查" does not.
    const options = menu.querySelectorAll('[data-testid="chat-input-skill-option"]');
    expect(options).toHaveLength(1);
    expect(options[0].getAttribute('data-skill-id')).toBe('calculator');
    // Description match is case-insensitive too ("Arithmetic").
    type('/ARITH');
    expect(
      menu.querySelectorAll('[data-testid="chat-input-skill-option"]'),
    ).toHaveLength(1);
    // Non-matching filter leaves the empty state, not a crash.
    type('/zzz-no-match');
    expect(menu.querySelector('[data-testid="chat-input-skill-empty"]')).not.toBeNull();
  });

  it('ArrowDown/ArrowUp move the data-active highlight; Enter selects without sending', () => {
    seedSkills();
    let sent = 0;
    render(<ChatInput onSend={() => { sent += 1; }} />);
    type('/');
    const menu = screen.getByTestId('chat-input-skill-menu');
    const options = () => menu.querySelectorAll('[data-testid="chat-input-skill-option"]');
    // Initial highlight on the first option.
    expect(options()[0].getAttribute('data-active')).toBe('true');
    expect(options()[1].getAttribute('data-active')).not.toBe('true');

    const textarea = screen.getByTestId('chat-input-textarea');
    fireEvent.keyDown(textarea, { key: 'ArrowDown' });
    expect(options()[0].getAttribute('data-active')).not.toBe('true');
    expect(options()[1].getAttribute('data-active')).toBe('true');
    fireEvent.keyDown(textarea, { key: 'ArrowUp' });
    expect(options()[0].getAttribute('data-active')).toBe('true');

    // Enter selects the highlighted skill: store written, input cleared,
    // menu closed, onSend NOT invoked.
    fireEvent.keyDown(textarea, { key: 'Enter' });
    expect(useAppStore.getState().currentSkillId).toBe('calculator');
    expect(
      (screen.getByTestId('chat-input-textarea') as HTMLTextAreaElement).value,
    ).toBe('');
    expect(screen.queryByTestId('chat-input-skill-menu')).toBeNull();
    expect(sent).toBe(0);
    // Badge immediately reflects the selection.
    expect(screen.getByTestId('chat-input-current-skill').textContent).toContain('Calculator');
  });

  it('Esc closes the menu keeping the text; outside mousedown also closes', () => {
    seedSkills();
    render(<ChatInput onSend={() => undefined} />);
    type('/cal');
    const textarea = screen.getByTestId('chat-input-textarea');
    fireEvent.keyDown(textarea, { key: 'Escape' });
    expect(screen.queryByTestId('chat-input-skill-menu')).toBeNull();
    expect((textarea as HTMLTextAreaElement).value).toBe('/cal');

    // Reopen, then close via outside mousedown.
    type('/');
    expect(screen.getByTestId('chat-input-skill-menu')).not.toBeNull();
    fireEvent.mouseDown(document.body);
    expect(screen.queryByTestId('chat-input-skill-menu')).toBeNull();
    // Text preserved on outside-close as well.
    expect((textarea as HTMLTextAreaElement).value).toBe('/');
  });

  it('renders 尚未安装任何 Skill when store.skills is empty (no crash)', () => {
    render(<ChatInput onSend={() => undefined} />);
    type('/');
    const menu = screen.getByTestId('chat-input-skill-menu');
    expect(
      menu.querySelectorAll('[data-testid="chat-input-skill-option"]'),
    ).toHaveLength(0);
    expect(screen.getByText('尚未安装任何 Skill')).toBeInTheDocument();
  });

  it('wizard mode (modeLabel) never opens the menu on "/"', () => {
    seedSkills();
    render(<ChatInput onSend={() => undefined} modeLabel="Skill 向导" />);
    type('/');
    expect(screen.queryByTestId('chat-input-skill-menu')).toBeNull();
  });

  it('Plan 05-08 WR-02 — clears currentSkillId when the selected skill disappears from store.skills', async () => {
    useAppStore.setState({
      skills: [{ id: 's1', name: 'S1', description: 'd', status: 'ok' }],
      currentSkillId: 's1',
    });
    render(<ChatInput onSend={() => undefined} />);
    expect(screen.getByTestId('chat-input-current-skill').textContent).toContain('S1');

    // Backend removes the Skill (reload-failed YAML, manual delete, etc.).
    useAppStore.setState({ skills: [] });

    await waitFor(() => {
      expect(useAppStore.getState().currentSkillId).toBeNull();
    });
    expect(screen.getByTestId('chat-input-current-skill').textContent).toContain('未选择 Skill');
  });

  it('Enter sends when the menu is NOT open (unchanged legacy behavior)', () => {
    seedSkills();
    let sent: string[] = [];
    render(<ChatInput onSend={(t) => { sent.push(t); }} />);
    type('普通问题');
    fireEvent.keyDown(screen.getByTestId('chat-input-textarea'), { key: 'Enter' });
    expect(sent).toEqual(['普通问题']);
  });
});

// within_text — minimal containment helper (avoids re-importing `within`
// for plain DOM text assertions on a menu element).
function within_text(container: HTMLElement, text: string): boolean {
  return container.textContent?.includes(text) ?? false;
}
