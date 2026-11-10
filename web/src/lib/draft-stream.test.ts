// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// answerStarted —— 「预览 vs 正式答案」替换判定的单元测试。
//
// 钉死的不变量：
//   1. 最后一条消息是 assistant 且已有非空正文 → true（预览卸载）。
//   2. 最后一条是 user（刚提交）→ false（预览必须继续显示）。
//   3. 倒数第二条 assistant 带上一轮正文，但最后一条是 user → false
//      （多轮对话里用旧消息判定会把新预览瞬间误杀）。
//   4. 只有空白正文 / 只有 reasoning 或 tool part → false。
//   5. 空消息列表 → false。
import { describe, expect, it } from 'vitest';
import type { UIMessage } from 'ai';
import { answerStarted } from './draft-stream';

function textMessage(id: string, role: 'user' | 'assistant', text: string): UIMessage {
  return { id, role, parts: [{ type: 'text', text }] } as UIMessage;
}

function reasoningMessage(id: string, role: 'user' | 'assistant'): UIMessage {
  return {
    id,
    role,
    parts: [{ type: 'reasoning', text: '思考中' }],
  } as UIMessage;
}

describe('answerStarted', () => {
  it('returns false for an empty message list', () => {
    expect(answerStarted([])).toBe(false);
  });

  it('returns true once the trailing assistant message has visible text', () => {
    const messages = [
      textMessage('u-1', 'user', '年假怎么折算？'),
      textMessage('a-1', 'assistant', '依据制度第 3 条'),
    ];
    expect(answerStarted(messages)).toBe(true);
  });

  it('returns false while the trailing assistant message has no text yet', () => {
    const messages = [
      textMessage('u-1', 'user', '年假怎么折算？'),
      reasoningMessage('a-1', 'assistant'),
    ];
    expect(answerStarted(messages)).toBe(false);
  });

  it('returns false when the trailing assistant text is whitespace only', () => {
    const messages = [textMessage('a-1', 'assistant', '   \n  ')];
    expect(answerStarted(messages)).toBe(false);
  });

  it('ignores the previous turn’s assistant text when the last message is the new user turn', () => {
    const messages = [
      textMessage('u-1', 'user', '第一轮问题'),
      textMessage('a-1', 'assistant', '第一轮的完整回答'),
      textMessage('u-2', 'user', '第二轮问题'),
    ];
    expect(answerStarted(messages)).toBe(false);
  });

  it('returns false when the trailing user message is followed by nothing', () => {
    expect(answerStarted([textMessage('u-1', 'user', '问题')])).toBe(false);
  });
});
