// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 41-05 — 知识图谱三元组查询视图（原 SPARQL Playground）。
//
// 命名与端点跟随后端的一次改名（前端必须严格一致）：
//   - 组件 SparqlPlaygroundView → KGTripleQueryView
//   - 端点   /api/sparql        → /api/kg/triples
//   - 类型   SparqlPattern/Result/Response → TriplePattern /
//            KGTripleResult / KGTripleResponse
//
// 为什么不再叫 SPARQL：这个视图实际做的事只是"subject / predicate /
// object 三元组模式匹配 + 过滤"，后端并没有实现 SPARQL 查询引擎。沿用
// SPARQL 的名字会让用户以为这里能写 SPARQL 查询语句，是不实的功能
// 宣称 —— 所以界面文案一律改成"三元组模式查询"。
//
// CodeMirror 6 editor for triple-pattern queries, skill selector,
// limit control, Run Query / Clear buttons, results table with
// provenance column.
//
// 2610 redesign: Run Query / Clear migrated to the shared Button, the
// results table to the shared DataTable, the provenance badge to the
// shared SourceKindBadge, the loading skeleton to LoadingSkeleton.
// Query interaction (CodeMirror mount, Ctrl+Enter, POST via
// kgTripleQuery with X-Skill-Name) is unchanged.

import { useEffect, useRef, useState } from 'react';
import { EditorView, basicSetup } from 'codemirror';
import { EditorState } from '@codemirror/state';
import { json } from '@codemirror/lang-json';
import { Search, Trash } from 'lucide-react';
import DataTable from '../shared/DataTable';
import { LoadingSkeleton } from '../shared/States';
import { SourceKindBadge } from '../shared/SourceKindBadge';
import { Button } from '../../ui/button';
import { kgTripleQuery } from './platformApi';
import type { KGTripleResponse, TriplePattern } from './types';

const DEFAULT_PATTERN = { s: '', p: 'rdf:type', o: 'skf:Skill', limit: 50 };
const LIMIT_OPTIONS = [10, 25, 50, 100, 500];

interface Props {
  skillName: string;
}

function ProvenanceCell({ prov }: { prov?: { source_kind: string; source_path?: string; line_start?: number; line_end?: number } | undefined }) {
  if (!prov) return <span className="text-text-muted">—</span>;
  return (
    <span className="flex flex-col gap-0.5">
      <SourceKindBadge kind={prov.source_kind} />
      {prov.source_path && (
        <span className="truncate text-[11px] max-w-xs" title={prov.source_path}>
          {prov.source_path}
        </span>
      )}
      {prov.line_start != null && prov.line_start > 0 && (
        <span className="text-[11px] text-text-muted">
          L{prov.line_start}
          {prov.line_end != null && prov.line_end > 0 ? `–L${prov.line_end}` : ''}
        </span>
      )}
    </span>
  );
}

export default function KGTripleQueryView({ skillName }: Props) {
  const editorRef = useRef<HTMLDivElement>(null);
  const editorViewRef = useRef<EditorView | null>(null);

  const [limit, setLimit] = useState(50);
  const [response, setResponse] = useState<KGTripleResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Mount CodeMirror editor
  useEffect(() => {
    if (!editorRef.current) return;

    const view = new EditorView({
      state: EditorState.create({
        doc: JSON.stringify(DEFAULT_PATTERN, null, 2),
        extensions: [
          basicSetup,
          json(),
          EditorView.theme({
            '&': { minHeight: '256px', fontSize: '12px' },
            '.cm-scroller': { fontFamily: 'var(--font-mono)' },
          }),
        ],
      }),
      parent: editorRef.current,
    });

    editorViewRef.current = view;

    return () => {
      view.destroy();
    };
  }, []);

  // Ctrl+Enter in editor — must re-attach when skillName/limit change
  useEffect(() => {
    const view = editorViewRef.current;
    if (!view) return;
    const handler = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
        e.preventDefault();
        // Read current doc + limit from DOM state
        const doc = view.state.doc.toString();
        let pattern: TriplePattern;
        try {
          pattern = { ...DEFAULT_PATTERN, ...JSON.parse(doc), limit };
        } catch {
          return;
        }
        setLoading(true);
        setError(null);
        kgTripleQuery(pattern, skillName)
          .then(setResponse)
          .catch((err: unknown) => {
            setError(err instanceof Error ? err.message : String(err));
            setResponse(null);
          })
          .finally(() => setLoading(false));
      }
    };
    view.dom.addEventListener('keydown', handler);
    return () => view.dom.removeEventListener('keydown', handler);
  }, [skillName, limit]);

  async function runQuery() {
    if (!editorViewRef.current) return;
    let pattern: TriplePattern;
    try {
      pattern = JSON.parse(editorViewRef.current.state.doc.toString());
    } catch {
      setError('查询格式错误：JSON 解析失败');
      return;
    }
    pattern.limit = limit;

    setLoading(true);
    setError(null);
    try {
      const resp = await kgTripleQuery(pattern, skillName);
      setResponse(resp);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : String(e));
      setResponse(null);
    } finally {
      setLoading(false);
    }
  }

  function clearResults() {
    if (!editorViewRef.current) return;
    editorViewRef.current.dispatch({
      changes: {
        from: 0,
        to: editorViewRef.current.state.doc.length,
        insert: JSON.stringify(DEFAULT_PATTERN, null, 2),
      },
    });
    setResponse(null);
    setError(null);
  }

  return (
    <div className="space-y-4" data-testid="platform-view-triples-body">
      {/* 三元组模式查询说明 —— 说清楚这个面板实际能做什么，
          避免用户以为它是完整的 SPARQL 引擎。 */}
      <p className="text-xs text-text-muted">
        三元组模式查询：subject / predicate / object 留空表示通配匹配，
        可选填 filter 做进一步过滤。
      </p>

      {/* CodeMirror editor */}
      <div
        ref={editorRef}
        className="min-h-64 rounded-md border border-border bg-surface-2 font-mono text-xs"
        aria-label="三元组模式编辑器"
      />

      {/* Controls row */}
      <div className="flex items-center gap-2">
        <select
          value={limit}
          onChange={(e) => setLimit(Number(e.target.value))}
          className="rounded-md border border-border bg-surface-2 px-2 py-1.5 text-xs"
        >
          {LIMIT_OPTIONS.map((n) => (
            <option key={n} value={n}>{n}</option>
          ))}
        </select>

        <Button type="button" onClick={runQuery} disabled={loading} size="sm">
          <Search className="h-3.5 w-3.5" />
          运行查询
        </Button>

        <Button type="button" onClick={clearResults} variant="secondary" size="sm">
          <Trash className="h-3.5 w-3.5" />
          清空
        </Button>
      </div>

      {/* Error */}
      {error && (
        <div className="text-sm text-danger">
          查询失败：{error}
          <button onClick={runQuery} className="ml-2 underline">
            （点击重试）
          </button>
        </div>
      )}

      {/* Results */}
      {response && (
        <div className="space-y-2">
          <div className="font-mono text-xs text-text-muted">
            结果：{response.count} 条三元组{response.truncated ? '（已截断）' : ''}
          </div>
          <DataTable
            columns={[
              {
                key: 's',
                label: 'subject',
                render: (r) => <span className="break-all font-mono">{r.s}</span>,
              },
              {
                key: 'p',
                label: 'predicate',
                render: (r) => (
                  <span className="break-all font-mono text-text-muted">{r.p}</span>
                ),
              },
              {
                key: 'o',
                label: 'object',
                render: (r) => <span className="break-all font-mono">{r.o}</span>,
              },
              {
                key: 'provenance',
                label: '来源',
                render: (r) => <ProvenanceCell prov={r.provenance} />,
              },
            ]}
            rows={response.results}
            rowKey={(_, i) => String(i)}
            empty={{ title: '输入三元组模式并点击「运行查询」' }}
          />
        </div>
      )}

      {/* Loading skeleton */}
      {loading && <LoadingSkeleton rows={3} />}
    </div>
  );
}
