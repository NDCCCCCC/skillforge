// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 41-05 — SPARQL Playground view.
// CodeMirror 6 editor for triple-pattern queries, skill selector,
// limit control, Run Query / Clear buttons, results table with
// provenance column.

import { useEffect, useRef, useState } from 'react';
import { EditorView, basicSetup } from 'codemirror';
import { EditorState } from '@codemirror/state';
import { json } from '@codemirror/lang-json';
import { Search, Trash } from 'lucide-react';
import { cn } from '../../../lib/cn';
import { sparqlQuery } from './platformApi';
import type { SparqlPattern, SparqlResponse } from './types';

const DEFAULT_PATTERN = { s: '', p: 'rdf:type', o: 'skf:Skill', limit: 50 };
const LIMIT_OPTIONS = [10, 25, 50, 100, 500];

interface Props {
  skillName: string;
  onSkillChange: (skill: string) => void;
}

function ProvenanceCell({ prov }: { prov?: { source_kind: string; source_path?: string; line_start?: number; line_end?: number } | undefined }) {
  if (!prov) return <span className="text-text-muted">—</span>;
  return (
    <span className="flex flex-col gap-0.5">
      <span
        className={cn(
          'w-fit text-[11px] font-semibold rounded px-1',
          prov.source_kind === 'file'
            ? 'bg-status-compliant/10 text-status-compliant'
            : prov.source_kind === 'emitter'
              ? 'bg-surface-3 text-text-muted'
              : 'bg-primary-dim text-primary',
        )}
      >
        {prov.source_kind === 'file' ? '文件' : prov.source_kind === 'emitter' ? '生成' : '归纳'}
      </span>
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

export default function SparqlPlaygroundView({ skillName, onSkillChange }: Props) {
  const editorRef = useRef<HTMLDivElement>(null);
  const editorViewRef = useRef<EditorView | null>(null);

  const [limit, setLimit] = useState(50);
  const [response, setResponse] = useState<SparqlResponse | null>(null);
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
        let pattern: SparqlPattern;
        try {
          pattern = { ...DEFAULT_PATTERN, ...JSON.parse(doc), limit };
        } catch {
          return;
        }
        setLoading(true);
        setError(null);
        sparqlQuery(pattern, skillName)
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
    let pattern: SparqlPattern;
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
      const resp = await sparqlQuery(pattern, skillName);
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
    <div className="space-y-4" data-testid="platform-view-sparql">
      {/* CodeMirror editor */}
      <div
        ref={editorRef}
        className="min-h-64 rounded-md border border-border bg-surface-2 font-mono text-xs"
        aria-label="SPARQL pattern editor"
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

        <button
          type="button"
          onClick={runQuery}
          disabled={loading}
          className="inline-flex items-center gap-1 rounded-md bg-primary px-3 py-1.5 text-xs font-semibold text-primary-fg disabled:opacity-50"
        >
          <Search className="h-3.5 w-3.5" />
          Run Query
        </button>

        <button
          type="button"
          onClick={clearResults}
          className="inline-flex items-center gap-1 rounded-md border border-border bg-surface-2 px-3 py-1.5 text-xs font-semibold hover:bg-surface-3"
        >
          <Trash className="h-3.5 w-3.5" />
          Clear
        </button>
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
            Results: {response.count} triples{response.truncated ? ' (truncated)' : ''}
          </div>
          <div className="max-h-96 overflow-y-auto rounded-md border border-border">
            <table className="w-full text-left text-xs">
              <thead className="sticky top-0 bg-surface-2 text-text-muted">
                <tr>
                  <th className="px-3 py-2">subject</th>
                  <th className="px-3 py-2">predicate</th>
                  <th className="px-3 py-2">object</th>
                  <th className="px-3 py-2">来源</th>
                </tr>
              </thead>
              <tbody>
                {response.results.length === 0 ? (
                  <tr>
                    <td colSpan={4} className="px-3 py-4 text-center text-text-muted">
                      输入查询模式并点击 Run Query
                    </td>
                  </tr>
                ) : (
                  response.results.map((r, i) => (
                    <tr key={i} className="border-t border-border hover:bg-surface-2">
                      <td className="break-all px-3 py-1.5 font-mono">{r.s}</td>
                      <td className="break-all px-3 py-1.5 font-mono text-text-muted">{r.p}</td>
                      <td className="break-all px-3 py-1.5 font-mono">{r.o}</td>
                      <td className="px-3 py-1.5">
                        <ProvenanceCell prov={r.provenance} />
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Loading skeleton */}
      {loading && (
        <div className="space-y-2">
          {[1, 2, 3].map((i) => (
            <div key={i} className="h-10 animate-pulse rounded bg-surface-2" />
          ))}
        </div>
      )}
    </div>
  );
}
