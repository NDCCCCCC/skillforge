// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 41-05 — MCP Inspector view.
// Server status row, tools table with Test buttons,
// Test Invocation Panel with JSON params editor.
//
// 2610 redesign: mount-time status/tools loads migrated to the shared
// useAdminData hook, the tools table to the shared DataTable, Test /
// Invoke buttons to the shared Button, loading placeholders to
// LoadingSkeleton. Invoke interaction (POST invokeMcpTool, params
// editor lifecycle) is unchanged.

import { useEffect, useRef, useState } from 'react';
import { EditorView, basicSetup } from 'codemirror';
import { EditorState } from '@codemirror/state';
import { json } from '@codemirror/lang-json';
import { cn } from '../../../lib/cn';
import { useAdminData } from '../shared/useAdminData';
import DataTable from '../shared/DataTable';
import { LoadingSkeleton } from '../shared/States';
import { Button } from '../../ui/button';
import { invokeMcpTool } from './platformApi';
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
  // Mount-time loads via the shared hook (auto 'sf_admin_token' header).
  const {
    data: status,
    loading: statusLoading,
    error: statusError,
    reload: reloadStatus,
  } = useAdminData<McpServerStatus>('/api/admin/platform/mcp/status');
  const {
    data: tools,
    loading: toolsLoading,
    error: toolsError,
    reload: reloadTools,
  } = useAdminData<McpTool[]>('/api/admin/platform/mcp/tools');

  const [selectedTool, setSelectedTool] = useState<string | null>(null);
  const [invokeResult, setInvokeResult] = useState<McpInvokeResponse | null>(null);
  const [invokeLoading, setInvokeLoading] = useState(false);
  const [invokeError, setInvokeError] = useState<string | null>(null);

  const paramsEditorRef = useRef<HTMLDivElement>(null);
  const paramsViewRef = useRef<EditorView | null>(null);
  const selectedToolRef = useRef<McpTool | null>(null);

  // Mount / remount params editor when selected tool changes
  useEffect(() => {
    const tool = (tools ?? []).find((t) => t.name === selectedTool) ?? null;
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
          <Button variant="ghost" size="sm" onClick={reloadStatus} className="ml-2 underline">
            （点击重试）
          </Button>
        </div>
      ) : status ? (
        <div className="flex items-center gap-2 rounded-md border border-border bg-surface-2 p-3 font-mono text-xs">
          <StatusDot online={status.status === 'online'} />
          <span className={status.status === 'online' ? 'text-status-compliant' : 'text-status-non-compliant'}>
            {status.status === 'online' ? 'Online' : 'Offline'}
          </span>
          <span className="text-text-muted">· service_skill: {status.service_skill}</span>
        </div>
      ) : statusLoading ? (
        <LoadingSkeleton rows={1} />
      ) : null}

      {/* Tools table */}
      {toolsError ? (
        <div className="text-sm text-danger">
          工具列表加载失败
          <Button variant="ghost" size="sm" onClick={reloadTools} className="ml-2 underline">
            （点击重试）
          </Button>
        </div>
      ) : toolsLoading ? (
        <LoadingSkeleton rows={3} />
      ) : (
        <DataTable
          columns={[
            {
              key: 'name',
              label: 'tool_name',
              render: (t) => <span className="font-mono">{t.name}</span>,
            },
            {
              key: 'description',
              label: 'description',
              render: (t) => (
                <span className="block max-w-xs truncate" title={t.description}>
                  {t.description}
                </span>
              ),
            },
            {
              key: 'actions',
              label: 'actions',
              render: (t) => (
                <Button
                  type="button"
                  variant="secondary"
                  size="sm"
                  onClick={() => {
                    setSelectedTool(t.name);
                    setInvokeResult(null);
                    setInvokeError(null);
                  }}
                >
                  Test
                </Button>
              ),
            },
          ]}
          rows={tools ?? []}
          rowKey={(t) => t.name}
          empty={{ title: '暂无工具' }}
        />
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

          <Button type="button" onClick={invokeTool} disabled={invokeLoading} size="sm">
            Invoke
          </Button>

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
