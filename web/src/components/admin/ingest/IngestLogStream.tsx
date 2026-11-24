// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Phase 55-02 (ING-05) -- the collapsible run log (RAGFlow mode;
// 55-UI-SPEC Component 4 运行日志 + §Honesty Rule 4).
//
// `lines` mirrors the forward-compat `progress_msg` wire field, which the
// 54 wire does not emit yet. Absent / empty -> the locked empty copy.
// Every line renders VERBATIM: the FE never parses a log line, never
// colors it, and never derives state from it (state comes exclusively
// from the step state machine).
import { useState } from 'react';

export interface IngestLogStreamProps {
  lines?: string[];
}

export default function IngestLogStream({ lines }: IngestLogStreamProps) {
  const [open, setOpen] = useState(false);
  const hasLines = Array.isArray(lines) && lines.length > 0;

  return (
    <section data-testid="ingest-log-stream" className="space-y-2">
      <div className="flex items-center justify-between gap-2">
        <h2 className="text-sm font-semibold text-text">运行日志</h2>
        <button
          type="button"
          data-testid="ingest-log-toggle"
          onClick={() => setOpen((b) => !b)}
          className="text-xs text-text-muted hover:text-text"
        >
          {open ? '收起' : '展开'}
        </button>
      </div>
      {open && (
        <div
          data-testid="ingest-log-body"
          className="max-h-48 overflow-y-auto rounded-md border border-border bg-surface-2 p-3"
        >
          {hasLines ? (
            lines.map((line, i) => (
              <pre
                key={i}
                data-testid="ingest-log-line"
                className="whitespace-pre-wrap font-mono text-xs text-text-dim"
              >
                {line}
              </pre>
            ))
          ) : (
            <p data-testid="ingest-log-empty" className="text-xs text-text-muted">
              本任务暂无过程日志
            </p>
          )}
        </div>
      )}
    </section>
  );
}
