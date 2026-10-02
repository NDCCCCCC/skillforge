// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// InspectorToolCall — per-tool-call row in the AgentInspector.
//
// Renders tool name + state + (collapsible) JSON-stringified args +
// JSON-stringified output. Tool args are rendered as text via React's
// default text-node rendering — NEVER via dangerouslySetInnerHTML
// (threat T-5-21 mitigation).

import { useState } from 'react';
import { ChevronDown, ChevronRight } from 'lucide-react';
import type { ToolCallRow } from './hop-types';

function safeStringify(value: unknown): string {
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

export interface InspectorToolCallProps {
  toolCall: ToolCallRow;
}

export default function InspectorToolCall({ toolCall }: InspectorToolCallProps) {
  const [expanded, setExpanded] = useState(false);
  const stateLabel = toolCall.errorText ? 'error' : toolCall.state;

  return (
    <details
      data-testid="inspector-tool-call"
      data-tool-name={toolCall.toolName}
      data-tool-call-id={toolCall.toolCallId}
      data-state={stateLabel}
      open={expanded}
      className="rounded-md border border-border bg-surface-2"
    >
      <summary
        className="flex cursor-pointer items-center gap-2 px-2 py-1 text-xs"
        onClick={(e) => {
          e.preventDefault();
          setExpanded((b) => !b);
        }}
      >
        {expanded ? (
          <ChevronDown className="h-3 w-3 text-text-muted" />
        ) : (
          <ChevronRight className="h-3 w-3 text-text-muted" />
        )}
        <span className="font-mono font-semibold text-primary">{toolCall.toolName}</span>
        <span className="text-text-muted">·</span>
        <span className="text-text-muted">{stateLabel}</span>
      </summary>
      <div className="flex flex-col gap-2 border-t border-border p-2">
        {toolCall.input !== undefined && (
          <div data-testid="inspector-tool-call-args">
            <div className="text-[10px] uppercase tracking-wider text-text-dim">
              args
            </div>
            <pre className="mt-1 overflow-x-auto whitespace-pre-wrap font-mono text-xs text-text">
              <code>{safeStringify(toolCall.input)}</code>
            </pre>
          </div>
        )}
        {toolCall.output !== undefined && (
          <div data-testid="inspector-tool-call-output">
            <div className="text-[10px] uppercase tracking-wider text-text-dim">
              output
            </div>
            <pre className="mt-1 overflow-x-auto whitespace-pre-wrap font-mono text-xs text-text">
              <code>{safeStringify(toolCall.output)}</code>
            </pre>
          </div>
        )}
        {toolCall.errorText && (
          <div
            data-testid="inspector-tool-call-error"
            className="font-mono text-xs text-danger"
          >
            {toolCall.errorText}
          </div>
        )}
      </div>
    </details>
  );
}