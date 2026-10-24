// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 41-05 — SPARQL Playground view.
// CodeMirror 6 editor for triple-pattern queries, skill selector,
// limit control, Run Query / Clear buttons, results table with
// provenance column.
//
// 2610 redesign: Run Query / Clear migrated to the shared Button, the
// results table to the shared DataTable, the provenance badge to the
// shared SourceKindBadge, the loading skeleton to LoadingSkeleton.
// Query interaction (CodeMirror mount, Ctrl+Enter, POST via
// sparqlQuery with X-Skill-Name) is unchanged.

import { useEffect, useRef, useState } from 'react';
import { EditorView, basicSetup } from 'codemirror';
import { EditorState } from '@codemirror/state';
import { json } from '@codemirror/lang-json';
import { Search, Trash } from 'lucide-react';
import DataTable from '../shared/DataTable';
import { LoadingSkeleton } from '../shared/States';
import { SourceKindBadge } from '../shared/SourceKindBadge';
import { Button } from '../../ui/button';
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

        <Button type="button" onClick={runQuery} disabled={loading} size="sm">
          <Search className="h-3.5 w-3.5" />
          Run Query
        </Button>

        <Button type="button" onClick={clearResults} variant="secondary" size="sm">
          <Trash className="h-3.5 w-3.5" />
          Clear
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
            Results: {response.count} triples{response.truncated ? ' (truncated)' : ''}
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
            empty={{ title: '输入查询模式并点击 Run Query' }}
          />
        </div>
      )}

      {/* Loading skeleton */}
      {loading && <LoadingSkeleton rows={3} />}
    </div>
  );
}
