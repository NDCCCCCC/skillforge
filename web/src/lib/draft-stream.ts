// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// draft-stream —— 「边生成边显示」的判定层（纯函数，零 React 依赖）。
//
// 背景（后端 wire 契约，internal/** 只读）：
//   internal/server/pipeline.go 的 streamAndAssemble 每收到一个 token 就
//   发一帧 {type:"text-delta-draft", id:"msg-1", delta:token}，流结束时发
//   {type:"text-delta-draft-end"}。正式答案随后才走
//   text-start / text-delta / text-end。
//
// 前端整条链路：
//   RealChatTransport（SSE dispatcher 拦下 draft 帧，不交给 useChat）
//     → onDraftUpdate({status, draftText})
//     → useAppStore.draftStatus / draftText
//     → ChatColumn 渲染 DraftStreamChip（流式预览）
//
// 这个模块只负责一件事：判断「正式答案是否已经开始流出」，也就是预览
// 该在什么时候被正式消息**替换**掉。两条不变量：
//   1. 正式答案的第一个字符到达 → 预览立刻消失。否则同一段话会以
//      「预览」+「正式消息」两种形态并存，用户会以为回答重复了。
//   2. 只看最后一条消息。多轮对话里倒数第二条 assistant 消息早就带着
//      上一轮正文，用它判定会让新问题的预览在提交瞬间就被误杀。
import type { UIMessage } from 'ai';

/** 一条消息里是否已有可见（非空白）的正文 text part。 */
function hasVisibleText(message: UIMessage): boolean {
  for (const part of message.parts) {
    if (part.type === 'text' && typeof part.text === 'string' && part.text.trim() !== '') {
      return true;
    }
  }
  return false;
}

/**
 * 正式答案是否已经开始渲染。
 *
 * useChat 在流式过程中逐 delta 改写最后一条 assistant 消息的 text part，
 * 所以正文第一个字符到达时这里立刻为 true —— 正是「预览换成正式答案」
 * 的那一刻。
 *
 * 最后一条不是 assistant（用户刚提交、占位消息还没插入）→ false，
 * 预览继续显示。
 */
export function answerStarted(messages: readonly UIMessage[]): boolean {
  const last = messages[messages.length - 1];
  if (!last || last.role !== 'assistant') return false;
  return hasVisibleText(last);
}
