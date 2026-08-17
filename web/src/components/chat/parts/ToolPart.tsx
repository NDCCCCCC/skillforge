// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// ToolPart — modernized tool-call card (Phase 4 visual upgrade).
//
// Renders tool-call lifecycle as an elevated card with:
//   - tool icon (🔧) in a colored badge
//   - tool name in mono font
//   - status dot (animated pulse for in-flight states)
//   - expandable input/output JSON (when present)
//   - state-driven left border color:
//     * in-flight (input-streaming / input-available): cyan + pulse
//     * success (output-available): green
//     * error (output-error / output-denied): red
//
// Per Pitfall 3, the discriminator is `tool-${name}` or `dynamic-tool`.
// For tools we didn't pre-list, MessageBubble's default arm catches
// `part.type.startsWith('tool-')` and forwards here.

import { Wrench } from 'lucide-react';
import type {
  DynamicToolUIPart,
  ToolUIPart,
  UIMessage,
} from 'ai';

type AnyToolPart = ToolUIPart | DynamicToolUIPart | (UIMessage['parts'][number] & { type: string; state?: string });

function getToolName(part: AnyToolPart): string {
  if (part.type === 'dynamic-tool') return 'dynamic-tool';
  return ((part as { toolName?: string }).toolName) ?? part.type;
}

function getState(part: AnyToolPart): string {
  return (part as { state?: string }).state ?? 'unknown';
}

function getStateLabel(state: string): string {
  switch (state) {
    case 'input-streaming':
      return '准备参数';
    case 'input-available':
      return '调用中';
    case 'output-available':
      return '已完成';
    case 'output-error':
      return '执行失败';
    case 'output-denied':
      return '已拒绝';
    default:
      return state;
  }
}

function safeStringify(value: unknown): string {
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

export function ToolPart({ part }: { part: AnyToolPart }) {
  const name = getToolName(part);
  const state = getState(part);
  const input = (part as { input?: unknown }).input;
  const output = (part as { output?: unknown }).output;
  const errorText = (part as { errorText?: unknown }).errorText;

  return (
    <div
      className="tool-card"
      data-testid="tool-part"
      data-tool-name={name}
      data-state={state}
    >
      <div className="tool-card-header">
        <span className="tool-card-icon" aria-hidden>
          <Wrench className="h-3 w-3" />
        </span>
        <span className="tool-card-name">{name}</span>
        <span className="tool-card-status">
          <span className="tool-card-status-dot" aria-hidden />
          <span>{getStateLabel(state)}</span>
        </span>
      </div>
      {input !== undefined && (
        <pre className="tool-card-body">
          <code>{safeStringify(input)}</code>
        </pre>
      )}
      {output !== undefined && (
        <pre className="tool-card-body">
          <code>{safeStringify(output)}</code>
        </pre>
      )}
      {errorText !== undefined && (
        <div className="tool-card-error">
          错误：{String(errorText)}
        </div>
      )}
    </div>
  );
}
