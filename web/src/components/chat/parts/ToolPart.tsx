// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// ToolPart — tool-call card (Open Design redesign B5: default collapsed).
//
// Renders the tool-call lifecycle as a compact status strip:
//   - 2px status-colored left border (in-flight → indigo + pulse,
//     success → green, error/denied → red)
//   - head: tool icon + name (mono) + status label + expand chevron
//   - input/output JSON bodies are HIDDEN until the user expands the
//     card (B5: the chat stream stays readable; the full payload is
//     one click away). errorText stays visible even collapsed — a
//     failed tool call is a loud-failure signal (IN-04 posture).
//
// Per Pitfall 3, the discriminator is `tool-${name}` or `dynamic-tool`.
// For tools we didn't pre-list, MessageBubble's default arm catches
// `part.type.startsWith('tool-')` and forwards here.

import { useState } from 'react';
import { ChevronDown, Wrench } from 'lucide-react';
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
  // B5: bodies default hidden. No toggle when there is nothing to show.
  const [expanded, setExpanded] = useState(false);
  const hasBodies = input !== undefined || output !== undefined;

  return (
    <div
      className="tool-card"
      data-testid="tool-part"
      data-tool-name={name}
      data-state={state}
      data-expanded={String(expanded)}
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
        {hasBodies && (
          <button
            type="button"
            data-testid="tool-toggle"
            onClick={() => setExpanded((b) => !b)}
            aria-expanded={expanded}
            className="tool-card-toggle"
          >
            <ChevronDown
              className="h-3 w-3"
              style={{ transform: expanded ? undefined : 'rotate(-90deg)' }}
            />
            <span>{expanded ? '收起' : '展开'}</span>
          </button>
        )}
      </div>
      {expanded && input !== undefined && (
        <pre className="tool-card-body">
          <code>{safeStringify(input)}</code>
        </pre>
      )}
      {expanded && output !== undefined && (
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
