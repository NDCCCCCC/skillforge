// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// DraftStreamChip —— 「边生成边显示」的流式预览区。
//
// 数据来源：后端 pipeline.go 的 streamAndAssemble 每收到一个 token 就发
// 一帧 text-delta-draft，前端 RealChatTransport 的 SSE dispatcher 把它拦
// 下来（绝不交给 useChat，否则会被文本 parser 当成正式正文），通过
// onDraftUpdate 写进 store；ChatColumn 读 store 渲染本组件。
//
// 渲染形态（2026-11-10 改造，对齐 pi / codex / dsh 的体感）：
//   - 卡片式预览区，不再是单行胶囊：正文 pre-wrap 多行显示、最高
//     40vh、超出内部滚动并自动贴底 —— 用户看到的就是模型正在吐的字。
//   - pending 时末尾跟一个闪烁光标，表示「还没写完」。
//   - verified 时转绿、去掉光标，表示接地校验已过、正式答案即将接管。
//   - refused 直接返回 null，由 RefusalCard 接管（不留孤儿预览）。
//
// 关于「打字机效果」：这里刻意不做 JS 字符级缓冲/延迟。draft 帧本身就是
// 一个 token 一帧，正文天然逐字增长；再叠一层定时器只会让人多等，与
// 「不要干等几十秒」的初衷相反。逐字节奏由后端 token 速率决定，光标负责
// 把「仍在生成」这件事讲清楚。
//
// 替换语义（不并存、不重复）：正式答案的 text-delta 一到达，ChatColumn
// 就用 answerStarted(messages) 卸载本组件，由正式的 assistant 消息接管
// —— 判定逻辑见 lib/draft-stream.ts。
//
// Status contract（既有测试钉死，勿改）：
//   - 'pending'  → 「正在生成回答」+ 累积文本 + 光标
//   - 'verified' → 「已验证」+ 累积文本（保留，连续性）
//   - 'refused'  → null
import { useEffect, useRef } from 'react';

/**
 * 预览最多渲染的尾部字符数。超长回答（几十 KB）没必要整段进 DOM：
 * 预览只是过程可视，正式答案会完整落地在 assistant 消息里。
 */
const MAX_PREVIEW_CHARS = 6000;

export interface DraftStreamChipProps {
  status: 'pending' | 'verified' | 'refused';
  draftText: string;
}

export default function DraftStreamChip({ status, draftText }: DraftStreamChipProps) {
  const bodyRef = useRef<HTMLDivElement>(null);
  const streaming = status === 'pending';
  const truncated = draftText.length > MAX_PREVIEW_CHARS;
  const text = truncated ? draftText.slice(-MAX_PREVIEW_CHARS) : draftText;

  // 自动贴底：每来一个新 token 就把预览滚到底部，用户永远看到最新一个字。
  useEffect(() => {
    const el = bodyRef.current;
    if (!el) return;
    el.scrollTop = el.scrollHeight;
  }, [text]);

  if (status === 'refused') return null;
  const label = streaming ? '正在生成回答' : '已验证';

  return (
    <div
      data-testid="draft-chip"
      data-status={status}
      className="draft-chip-modern"
      aria-busy={streaming}
    >
      <div className="draft-chip-head">
        <span className="draft-chip-dot" aria-hidden />
        {/* live region 只包住这句短状态文案：正文逐字增长不该被反复朗读。 */}
        <span className="draft-chip-label" aria-live="polite">
          {label}
        </span>
        <span className="draft-chip-meta" aria-hidden>
          {streaming ? '流式输出' : '已定稿'}
        </span>
      </div>
      {draftText && (
        <div className="draft-chip-body" ref={bodyRef} data-testid="draft-scroll">
          {truncated && (
            <span className="draft-chip-ellipsis" aria-hidden>
              …（前面内容已折叠，仅显示最近 {MAX_PREVIEW_CHARS} 字）
            </span>
          )}
          <span className="draft-chip-text" data-testid="draft-text">
            {text}
          </span>
          {streaming && <span className="draft-chip-caret" data-testid="draft-caret" aria-hidden />}
        </div>
      )}
    </div>
  );
}
