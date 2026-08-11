// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// ToolPart — renders tool-call lifecycle (input-streaming →
// input-available → output-available / output-error / output-denied).
// Per Pitfall 3, the discriminator is `tool-${name}` or `dynamic-tool`
// (NOT `tool-invocation` — that name doesn't exist in AI SDK v7).
//
// For tools we didn't pre-list, MessageBubble's default arm catches
// `part.type.startsWith('tool-')` and forwards here. This component
// therefore accepts any tool-* part via a structural type.
//
// Wave 1 ships a minimal renderer (tool name + JSON-stringified input/
// output). Wave 3 expands with per-tool custom views if needed.

import type {
  DynamicToolUIPart,
  ToolUIPart,
  UIMessage,
} from 'ai';

type AnyToolPart = ToolUIPart | DynamicToolUIPart | (UIMessage['parts'][number] & { type: string; state?: string });

function getToolName(part: AnyToolPart): string {
  if (part.type === 'dynamic-tool') return 'dynamic-tool';
  return (part as ToolUIPart).toolName ?? part.type;
}

function getState(part: AnyToolPart): string {
  return (part as { state?: string }).state ?? 'unknown';
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
      className="rounded-md border border-border bg-surface-2 p-2 my-2 text-xs font-mono"
      data-testid="tool-part"
      data-tool-name={name}
      data-state={state}
    >
      <div className="flex items-center gap-2 text-text-muted">
        <span className="text-primary font-semibold">🔧 {name}</span>
        <span>·</span>
        <span>{state}</span>
      </div>
      {input !== undefined && (
        <pre className="mt-2 overflow-x-auto text-text">
          <code>{safeStringify(input)}</code>
        </pre>
      )}
      {output !== undefined && (
        <pre className="mt-2 overflow-x-auto text-text">
          <code>{safeStringify(output)}</code>
        </pre>
      )}
      {errorText !== undefined && (
        <div className="mt-2 text-danger">error: {String(errorText)}</div>
      )}
    </div>
  );
}
