// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// ChatColumn × draft 预览的替换语义（集成层）。
//
// 需求：正式答案到达后必须**替换**预览，绝不允许「预览」和正式消息同时
// 出现在同一条回答里 —— 否则同一段话在界面上重复一遍，用户会以为模型
// 说了两次。这里从 store 灌入 draft 状态，断言预览卡片的去留。
//
// 判定逻辑本身（answerStarted）的单元测试在 lib/draft-stream.test.ts。
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { cleanup, render } from '@testing-library/react';
import type { UIMessage } from 'ai';
import ChatColumn from './ChatColumn';
import { useAppStore } from '../../store/app-store';

function userMessage(id: string): UIMessage {
  return {
    id,
    role: 'user',
    parts: [{ type: 'text', text: '年假未休完怎么折算？' }],
  } as UIMessage;
}

function assistantMessage(id: string, text: string): UIMessage {
  return {
    id,
    role: 'assistant',
    parts: [{ type: 'text', text }],
  } as UIMessage;
}

function resetDraft(): void {
  useAppStore.getState().setDraft('idle', '');
}

beforeEach(() => {
  globalThis.localStorage.clear();
  resetDraft();
});

afterEach(() => {
  cleanup();
  resetDraft();
});

describe('ChatColumn — draft 预览的替换语义', () => {
  it('renders the preview while the draft streams and no answer text exists yet', () => {
    useAppStore.getState().setDraft('pending', '依据制度第 3 条，');
    const { container } = render(
      <ChatColumn messages={[userMessage('u-1')]} status="streaming" onSend={() => {}} />,
    );
    const chip = container.querySelector('[data-testid="draft-chip"]');
    expect(chip).not.toBeNull();
    expect(chip?.getAttribute('data-status')).toBe('pending');
  });

  it('unmounts the preview as soon as the real answer starts streaming (no duplicate text)', () => {
    useAppStore.getState().setDraft('pending', '依据制度第 3 条，未休年假按日工资 300% 折算');
    const { container } = render(
      <ChatColumn
        messages={[userMessage('u-1'), assistantMessage('a-1', '依据制度第 3 条：')]}
        status="streaming"
        onSend={() => {}}
      />,
    );
    // 正式答案的 text-delta 已经落地 → 预览必须消失，只留 assistant 消息。
    expect(container.querySelector('[data-testid="draft-chip"]')).toBeNull();
    expect(container.textContent).toContain('依据制度第 3 条');
  });

  it('renders nothing when the draft channel is idle', () => {
    const { container } = render(
      <ChatColumn messages={[userMessage('u-1')]} status="ready" onSend={() => {}} />,
    );
    expect(container.querySelector('[data-testid="draft-chip"]')).toBeNull();
  });

  it('keeps the preview for a new turn even though the previous answer has text', () => {
    useAppStore.getState().setDraft('pending', '新一轮的第一个字');
    const { container } = render(
      <ChatColumn
        messages={[
          userMessage('u-1'),
          assistantMessage('a-1', '上一轮的完整回答'),
          userMessage('u-2'),
        ]}
        status="streaming"
        onSend={() => {}}
      />,
    );
    expect(container.querySelector('[data-testid="draft-chip"]')).not.toBeNull();
  });
});
