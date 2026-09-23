// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 41-05 — MCP Inspector view.
// Server status row, tools table with Test buttons,
// Test Invocation Panel with JSON params editor.

import { useEffect, useRef, useState } from 'react';
import { EditorView, basicSetup } from 'codemirror';
import { EditorState } from '@codemirror/state';
import { json } from '@codemirror/lang-json';
import { cn } from '../../../lib/cn';
import { fetchMcpStatus, fetchMcpTools, invokeMcpTool } from './platformApi';
import type { McpInvokeResponse, McpServerStatus, McpTool } from './types';

function StatusDot({ online }: { online: boolean }) {
  return (
    <span
      className={cn(
        'inline-block h-2 w-2 rounded-full',
        online ? 'bg-status-compliant' : 'bg-status-non-compliant',
      )}
    />
  );
}

interface Props {}

export default function McpInspectorView(_props: Props) {
  const [status, setStatus] = useState<McpServerStatus | null>(null);
  const [tools, setTools] = useState<McpTool[]>([]);
  const [statusError, setStatusError] = useState<string | null>(null);
  const [toolsError, setToolsError] = useState<string | null>(null);

  const [selectedTool, setSelectedTool] = useState<string | null>(null);
  const [invokeResult, setInvokeResult] = useState<McpInvokeResponse | null>(null);
  const [invokeLoading, setInvokeLoading] = useState(false);
  const [invokeError, setInvokeError] = useState<string | null>(null);

  const paramsEditorRef = useRef<HTMLDivElement>(null);
  const paramsViewRef = useRef<EditorView | null>(null);
  const selectedToolRef = useRef<McpTool | null>(null);

  useEffect(() => {
    fetchMcpStatus()
      .then(setStatus)
      .catch((e: unknown) => setStatusError(e instanceof Error ? e.message : String(e)));
    fetchMcpTools()
      .then(setTools)
      .catch((e: unknown) => setToolsError(e instanceof Error ? e.message : String(e)));
  }, []);

  // Mount / remount params editor when selected tool changes
  useEffect(() => {
    const tool = tools.find((t) => t.name === selectedTool) ?? null;
    selectedToolRef.current = tool;

    if (paramsViewRef.current) {
      paramsViewRef.current.destroy();
      paramsViewRef.current = null;
    }

    if (!paramsEditorRef.current || !tool) return;

    // Generate skeleton params from inputSchema
    const skeleton = schemaToSkeleton(tool.inputSchema);
    const view = new EditorView({
      state: EditorState.create({
        doc: JSON.stringify(skeleton, null, 2),
        extensions: [
          basicSetup,
          json(),
          EditorView.theme({
            '&': { minHeight: '96px', fontSize: '12px' },
            '.cm-scroller': { fontFamily: 'var(--font-mono)' },
          }),
        ],
      }),
      parent: paramsEditorRef.current,
    });
    paramsViewRef.current = view;

    return () => {
      view.destroy();
    };
  }, [selectedTool, tools]);

  async function invokeTool() {
    if (!paramsViewRef.current || !selectedTool) return;
    let params: Record<string, unknown>;
    try {
      params = JSON.parse(paramsViewRef.current.state.doc.toString());
    } catch {
      setInvokeError('参数格式错误：JSON 解析失败');
      return;
    }
    setInvokeLoading(true);
    setInvokeError(null);
    setInvokeResult(null);
    try {
      const resp = await invokeMcpTool({ tool: selectedTool, params });
      setInvokeResult(resp);
    } catch (e: unknown) {
      setInvokeError(e instanceof Error ? e.message : String(e));
    } finally {
      setInvokeLoading(false);
    }
  }

  return (
    <div className="space-y-4" data-testid="platform-view-mcp">
      {/* Server status */}
      {statusError ? (
        <div className="text-sm text-danger">
          MCP 状态加载失败
          <button
            onClick={() => window.location.reload()}
            className="ml-2 underline"
          >
            （点击重试）
          </button>
        </div>
      ) : status ? (
        <div className="flex items-center gap-2 rounded-md border border-border bg-surface-2 p-3 font-mono text-xs">
          <StatusDot online={status.status === 'online'} />
          <span className={status.status === 'online' ? 'text-status-compliant' : 'text-status-non-compliant'}>
            {status.status === 'online' ? 'Online' : 'Offline'}
          </span>
          <span className="text-text-muted">· service_skill: {status.service_skill}</span>
        </div>
      ) : (
        <div className="h-10 animate-pulse rounded bg-surface-2" />
      )}

      {/* Tools table */}
      {toolsError ? (
        <div className="text-sm text-danger">
          工具列表加载失败
          <button
            onClick={() => window.location.reload()}
            className="ml-2 underline"
          >
            （点击重试）
          </button>
        </div>
      ) : (
        <div className="overflow-hidden rounded-md border border-border">
          <table className="w-full text-left text-xs">
            <thead className="bg-surface-2 text-text-muted">
              <tr>
                <th className="px-3 py-2">tool_name</th>
                <th className="px-3 py-2">description</th>
                <th className="px-3 py-2">actions</th>
              </tr>
            </thead>
            <tbody>
              {tools.length === 0 ? (
                <tr>
                  <td colSpan={3} className="px-3 py-4 text-center text-text-muted">
                    暂无工具
                  </td>
                </tr>
              ) : (
                tools.map((t) => (
                  <tr
                    key={t.name}
                    className="border-t border-border hover:bg-surface-2"
                  >
                    <td className="px-3 py-1.5 font-mono">{t.name}</td>
                    <td className="px-3 py-1.5 max-w-xs truncate" title={t.description}>
                      {t.description}
                    </td>
                    <td className="px-3 py-1.5">
                      <button
                        type="button"
                        onClick={() => {
                          setSelectedTool(t.name);
                          setInvokeResult(null);
                          setInvokeError(null);
                        }}
                        className="rounded border border-border bg-surface-2 px-2 py-1 text-xs font-semibold hover:bg-surface-3"
                      >
                        Test
                      </button>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      )}

      {/* Test Invocation Panel */}
      {selectedTool && (
        <div className="space-y-3 rounded-md border border-border bg-surface-2 p-3">
          <div className="text-xs font-semibold">tool: {selectedTool}</div>

          {/* Params editor */}
          <div
            ref={paramsEditorRef}
            className="rounded border border-border bg-surface font-mono text-xs"
            aria-label="MCP tool parameters editor"
          />

          <button
            type="button"
            onClick={invokeTool}
            disabled={invokeLoading}
            className="rounded bg-primary px-3 py-1.5 text-xs font-semibold text-primary-fg disabled:opacity-50"
          >
            Invoke
          </button>

          {/* Result area */}
          {invokeError && (
            <div className="text-sm text-danger">调用失败：{invokeError}</div>
          )}
          {invokeResult && (
            <div>
              {invokeResult.error ? (
                <div className="text-sm text-danger">调用失败：{invokeResult.error}</div>
              ) : invokeResult.result === undefined ? (
                <div className="text-sm text-text-muted">无返回结果</div>
              ) : (
                <pre className="max-h-64 overflow-auto rounded bg-surface p-3 font-mono text-xs">
                  {JSON.stringify(invokeResult.result, null, 2)}
                </pre>
              )}
            </div>
          )}
          {invokeLoading && (
            <div className="text-sm text-text-muted">调用中...</div>
          )}
        </div>
      )}
    </div>
  );
}

// ── Helpers ────────────────────────────────────────────────────────────────

function schemaToSkeleton(schema: Record<string, unknown>): Record<string, unknown> {
  const props = (schema['properties'] as Record<string, unknown>) ?? {};
  const required = (schema['required'] as string[]) ?? [];
  const result: Record<string, unknown> = {};
  for (const [key, val] of Object.entries(props)) {
    if (typeof val === 'object' && val !== null && 'type' in (val as object)) {
      const t = (val as { type: string }).type;
      result[key] =
        t === 'string' ? ''
          : t === 'number' || t === 'integer' ? 0
          : t === 'boolean' ? false
          : t === 'array' ? []
          : t === 'object' ? schemaToSkeleton(val as Record<string, unknown>)
          : null;
    } else {
      result[key] = null;
    }
    // Make required fields non-null so the JSON is valid
    if (result[key] === null && required.includes(key)) {
      result[key] = '';
    }
  }
  return result;
}
